# --- Standard Library Imports ---
import json
import os
import argparse
import asyncio
from typing import Type, Optional, Dict, Any, List
from pathlib import Path
import tempfile
from enum import Enum
import sys
import importlib.util # For dynamic module loading

from dotenv import load_dotenv
from langchain_openai import ChatOpenAI

from pydantic import BaseModel as PydanticBaseModel
# No need to import Field, AnyUrl etc. here for get_pydantic_model_from_schema's direct use

from browser_use import Agent, BrowserSession, Controller, ActionResult
from datamodel_code_generator import (
    generate,
    InputFileType,
    DataModelType,
)

# Default Summary model (used if no schema is provided or generation fails)
class Summary(PydanticBaseModel):
    summary: str


# --- Custom Actions ---

async def upload_file_action(
    index: int,
    path: str,
    browser_session: BrowserSession,
    available_file_paths: list[str],
):
    abs_path = os.path.abspath(path)
    available_abs_paths = [os.path.abspath(p) for p in available_file_paths]
    if abs_path not in available_abs_paths:
        return ActionResult(
            error=f"File path {path} (resolved: {abs_path}) is not in the allowed list: {available_abs_paths}"
        )

    el_info = await browser_session.find_file_upload_element_by_index(index)
    if not el_info:
        return ActionResult(error=f"No file-upload element at index {index}")
    handle = await browser_session.get_locate_element(el_info)
    try:
        await handle.set_input_files(path)
        # Ensure the ActionResult reflects what the LLM should understand as the outcome
        return ActionResult(extracted_content=f"Successfully uploaded file “{path}” to element at index {index}.", include_in_memory=True)
    except Exception as e:
        return ActionResult(error=f"Failed to upload file “{path}” at index {index}: {str(e)}")

# --- End Custom Actions ---


def parse_args():
    p = argparse.ArgumentParser()
    p.add_argument("--prompt", required=True, help="The LLM task prompt")
    p.add_argument(
        "--out", default="output/result.json", help="Where to write the JSON result"
    )
    p.add_argument(
        "--upload-file-paths",
        nargs="*",
        default=[],
        help="List of local file paths available for upload by the agent for this task",
    )
    p.add_argument(
        "--output-schema",
        type=str,
        default=None,
        help="A JSON string representing the JSON schema for the desired structured output.",
    )
    p.add_argument(
        "--model-name",
        type=str,
        default="DynamicOutputModel",
        help="The desired class name for the root Pydantic model generated from the schema.",
    )
    return p.parse_args()


def get_pydantic_model_from_schema(
    json_schema_str: str,
    model_name: str = "DynamicOutputModel" # This is the class_name passed to datamodel-codegen
) -> Type[PydanticBaseModel]:
    print(f"Attempting to generate and import Pydantic model '{model_name}' from schema...")
    with tempfile.NamedTemporaryFile(mode="w+", suffix=".py", delete=False) as tmp_model_file:
        output_path = Path(tmp_model_file.name)
        generated_model_class: Optional[Type[PydanticBaseModel]] = None
        # Ensure temp_dir is defined for finally block even if generate fails early
        temp_dir = str(output_path.parent)
        module_name_for_import = f"temp_dyn_model_{output_path.stem}" # Unique module name

        try:
            generate(
                json_schema_str,
                input_file_type=InputFileType.JsonSchema,
                input_filename="schema.json", # Dummy for string input
                output=output_path,
                output_model_type=DataModelType.PydanticV2BaseModel,
                class_name=model_name,
            )
            model_code = output_path.read_text()
            print(f"--- Generated Pydantic Model Code ({model_name}) ---\n{model_code}---------------------------------------\n")

            # Add the temporary directory to sys.path to allow importing the generated module
            if temp_dir not in sys.path:
                sys.path.insert(0, temp_dir)
                print(f"DEBUG: Added {temp_dir} to sys.path")

            spec = importlib.util.spec_from_file_location(module_name_for_import, str(output_path))
            if not spec or not spec.loader:
                raise ImportError(f"Could not create module spec for generated code at {output_path}")

            generated_module = importlib.util.module_from_spec(spec)
            # Before executing module, add its name to sys.modules to handle potential circular imports
            # within the generated code or its dependencies (though unlikely for simple models).
            sys.modules[module_name_for_import] = generated_module
            spec.loader.exec_module(generated_module)
            print(f"Successfully executed module: {module_name_for_import}")

            model_class_candidate = None
            # Try to get the model by the name we asked datamodel-code-generator to use
            if hasattr(generated_module, model_name) and \
               isinstance(getattr(generated_module, model_name), type) and \
               issubclass(getattr(generated_module, model_name), PydanticBaseModel):
                model_class_candidate = getattr(generated_module, model_name)
            else:
                # Fallback if the requested model_name isn't found (e.g., generator used schema title)
                for name, obj in vars(generated_module).items():
                    if isinstance(obj, type) and \
                       issubclass(obj, PydanticBaseModel) and \
                       obj is not PydanticBaseModel and obj is not Summary:
                        print(f"Warning: Model name '{model_name}' not found. Using first found Pydantic model: {name}")
                        model_class_candidate = obj
                        break
            
            if model_class_candidate:
                print(f"Successfully obtained Pydantic model candidate: {model_class_candidate.__name__} from module {module_name_for_import}")
                # With importlib, model_rebuild might not even be necessary as types are resolved
                # during standard module loading. But it doesn't hurt.
                model_class_candidate.model_rebuild(force=True)
                print(f"Called model_rebuild() on {model_class_candidate.__name__}")
                generated_model_class = model_class_candidate
            else:
                print(f"Error: Could not find or load any Pydantic model from generated module {module_name_for_import}.")
                
        except Exception as e:
            print(f"ERROR CAUGHT in get_pydantic_model_from_schema: {type(e).__name__}: {e}")
            if output_path.exists(): # output_path is defined at the start of the with block
                print(f"Generated code (if any) at: {output_path}")
        finally:
            # Clean up: remove module from sys.modules, remove temp dir from sys.path, delete file
            if module_name_for_import in sys.modules:
                del sys.modules[module_name_for_import]
                print(f"DEBUG: Removed {module_name_for_import} from sys.modules")
            if temp_dir in sys.path: # temp_dir should always be defined
                sys.path.remove(temp_dir)
                print(f"DEBUG: Removed {temp_dir} from sys.path")
            
            if output_path.exists(): # output_path should always be defined
                # Clean up .pyc file if it exists
                pyc_file = output_path.with_suffix('.pyc')
                if pyc_file.exists():
                    try: os.unlink(pyc_file)
                    except: pass # Ignore errors deleting .pyc
                try: os.unlink(output_path) # Delete the .py file
                except Exception as e_unlink: print(f"Warning: Could not delete temporary model file {output_path}: {e_unlink}")
        
        return generated_model_class if generated_model_class else Summary

async def main():
    args = parse_args()
    load_dotenv()

    output_model_class: Type[PydanticBaseModel] = Summary  # Default
    if args.output_schema:
        try:
            json.loads(args.output_schema)
            output_model_class = get_pydantic_model_from_schema(
                args.output_schema, args.model_name
            )
            print(f"Using output model: {output_model_class.__name__}")
        except json.JSONDecodeError as e:
            print(
                f"Error: --output-schema is not valid JSON: {e}. Defaulting to Summary model."
            )
            output_model_class = Summary # Ensure fallback
        except Exception as e: # Catch other errors from get_pydantic_model_from_schema
            print(
                f"Unexpected error processing --output-schema or generating model: {e}. Defaulting to Summary model."
            )
            output_model_class = Summary # Ensure fallback

    controller = Controller(output_model=output_model_class)

    # Register custom actions with the controller instance
    action_decorator = controller.action(
        "Uploads a file from the host system to a file input element on the current webpage. Requires the index of the file input element and the path of the file (from available_file_paths) to upload."
    )
    action_decorator(upload_file_action)

    llm_instance = ChatOpenAI(
        model="gpt-4o",
        temperature=0.3,
        api_key=os.getenv("OPENAI_API_KEY"),
    )

    # Consider making headless configurable via an arg 
    browser_session = BrowserSession(headless=True, user_data_dir=None) 

    agent = Agent(
        task=args.prompt,
        llm=llm_instance,
        controller=controller,
        browser_session=browser_session,
        available_file_paths=args.upload_file_paths,
    )

    try:
        history = await agent.run()
        result_json = history.final_result()

        os.makedirs(os.path.dirname(args.out), exist_ok=True)
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(result_json)
        print(f"Wrote result to {args.out}")

    except Exception as e:
        print(f"Error during agent run: {e}")
    finally:
        print("Done!")

if __name__ == "__main__":
    asyncio.run(main())