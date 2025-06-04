import json
import os
import argparse
import asyncio
from datetime import datetime, timezone
from typing import Type, Optional
from pathlib import Path
import tempfile
from enum import Enum
import sys
import importlib.util # For dynamic module loading

from dotenv import load_dotenv
from langchain_openai import ChatOpenAI

from pydantic import BaseModel as PydanticBaseModel

from browser_use import Agent, BrowserSession, Controller, ActionResult, BrowserProfile
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

async def get_last_downloaded_file_info(
    browser_session: BrowserSession,
) -> ActionResult:
    if not browser_session.browser_profile or not browser_session.browser_profile.downloads_dir:
        return ActionResult(error="Browser download directory not configured in session profile.")
    
    target_dir = Path(browser_session.browser_profile.downloads_dir)

    if not target_dir.exists():
        return ActionResult(error=f"Target download directory does not exist: {target_dir}")

    files_in_dir = [f for f in target_dir.iterdir() if f.is_file()]
    if not files_in_dir:
        return ActionResult(extracted_content="No files found in the target download directory.", include_in_memory=True)

    try:
        latest_file = max(files_in_dir, key=lambda f: f.stat().st_mtime)
    except ValueError:
         return ActionResult(extracted_content="No files to determine latest in target download directory.", include_in_memory=True)

    file_info_payload = {
        "actual_downloaded_filename": latest_file.name,
        "download_path": str(latest_file.resolve()), # Full path where it was saved
        "size_bytes": latest_file.stat().st_size,
        "modified_time_utc": datetime.fromtimestamp(latest_file.stat().st_mtime, tz=timezone.utc).isoformat(),
        "status": "confirmed_in_target_dir"
    }
    
    return ActionResult(
        extracted_content=f"Confirmed latest file in target download directory: {latest_file.name}",
        payload=file_info_payload, 
        include_in_memory=True
    )

async def click_and_wait_for_download(
    index: int, 
    browser_session: BrowserSession,
) -> ActionResult:
    print(f"DEBUG [C&W]: Entered action with index: {index}")
    if not browser_session.browser_profile or not browser_session.browser_profile.downloads_dir:
        print("DEBUG [C&W]: Error - Browser download directory not configured.")
        return ActionResult(error="Browser download directory not configured in session profile.")
    target_dir = Path(browser_session.browser_profile.downloads_dir)
    
    print(f"DEBUG [C&W]: Target download directory: {target_dir}")
    if not target_dir.exists():
        print(f"DEBUG [C&W]: Target directory {target_dir} does not exist. Attempting to create.")
        try:
            target_dir.mkdir(parents=True, exist_ok=True)
            print(f"DEBUG [C&W]: Created target_dir: {target_dir}")
        except Exception as e_mkdir:
            print(f"DEBUG [C&W]: Error creating target_dir: {e_mkdir}")
            return ActionResult(error=f"Target download directory '{target_dir}' does not exist and could not be created: {e_mkdir}")

    page = await browser_session.get_current_page()
    if not page:
        print("DEBUG [C&W]: Error - No active page.")
        return ActionResult(error="No active page found in browser session.")
    print(f"DEBUG [C&W]: Current page URL: {page.url}")

    element_handle = None
    el_info = None # Define el_info here so it's in scope for the print after try block
    try:
        print(f"DEBUG [C&W]: Getting browser state summary to find element by index {index}...")
        current_state_summary = await browser_session.get_state_summary(cache_clickable_elements_hashes=True) # Removed page=page
        
        if not current_state_summary or not current_state_summary.selector_map:
            print(f"DEBUG [C&W]: Error - Could not get state summary or selector map is empty.")
            return ActionResult(error="Failed to get page state or no interactive elements found.")

        el_info = current_state_summary.selector_map.get(index) # el_info is DOMElementNode
        
        # --- This is your safe debug print block ---
        debug_label = "N/A"
        debug_role = "N/A"
        debug_tag = "N/A"
        if el_info:
            print(f"DEBUG [C&W]: Raw el_info for index {index}: {el_info}")
            print(f"DEBUG [C&W]: Type of el_info: {type(el_info)}")
            # print(f"DEBUG [C&W]: dir(el_info): {dir(el_info)}") # Already saw this, it has 'attributes' and 'tag_name'

            if hasattr(el_info, 'attributes') and isinstance(el_info.attributes, dict):
                debug_label = el_info.attributes.get('aria-label', debug_label)
                debug_role = el_info.attributes.get('role', debug_role)
            # No direct 'aria_label' on DOMElementNode confirmed by error
        
            if hasattr(el_info, 'tag_name'): # DOMElementNode has tag_name
                debug_tag = el_info.tag_name
            elif hasattr(el_info, 'html_tag'): # Fallback if structure changes
                debug_tag = el_info.html_tag
        print(f"DEBUG [C&W]: Element info for index {index} (from safe access): Label='{debug_label}', Role='{debug_role}', Tag='{debug_tag}'")
        # --- End safe debug print block ---
        
        if not el_info: # Check el_info after trying to populate debug strings
            print(f"DEBUG [C&W]: Error - Element with index {index} not found in selector_map. Available indices: {list(current_state_summary.selector_map.keys())}")
            return ActionResult(error=f"Element with index {index} not found in current page's interactive elements.")
        
        element_handle = await browser_session.get_locate_element(el_info) # This should accept DOMElementNode
        if not element_handle:
            print(f"DEBUG [C&W]: Error - Could not get Playwright handle for element at index {index}.")
            # Use the safe debug_label here if needed for the error message
            return ActionResult(error=f"Could not get Playwright handle for element index {index} (tag: {debug_tag}, label: {debug_label}).")
        print(f"DEBUG [C&W]: Successfully got element handle for index {index}.")

    except Exception as e_find: # Catch any other unexpected error during element finding
        print(f"DEBUG [C&W]: Exception while finding element: {type(e_find).__name__}: {e_find}")
        return ActionResult(error=f"Error finding/locating element at index {index}: {str(e_find)}")

    # --- Perform click and wait for download (this part should now be reached) ---
    try:
        print(f"DEBUG [C&W]: Waiting for download to start after clicking element at index {index}...")
        async with page.expect_download(timeout=30000) as download_info_ctx:
            click_target_text = "N/A"
            try:
                click_target_text = await element_handle.text_content(timeout=1000)
            except:
                pass # Ignore if text_content fails, not critical for click
            print(f"DEBUG [C&W]: About to click element with text hint: '{click_target_text[:50]}...'")
            await element_handle.click(timeout=10000) 
            print(f"DEBUG [C&W]: Click action initiated for element at index {index}.")
        
        download = await download_info_ctx.value 
        print(f"DEBUG [C&W]: Download event received. Suggested filename: {download.suggested_filename}, URL: {download.url}")
        temp_download_path = await download.path()
        print(f"DEBUG [C&W]: Playwright temporary download path: {temp_download_path}")

        suggested_filename = download.suggested_filename
        save_path = target_dir / suggested_filename
        
        print(f"DEBUG [C&W]: Attempting to save download to: {save_path}")
        await download.save_as(save_path)
        print(f"DEBUG [C&W]: Called download.save_as({save_path})")

        if not save_path.is_file() or save_path.stat().st_size == 0:
            print(f"ERROR [C&W]: File not found or empty at {save_path} after save_as. Original suggested: {suggested_filename}")
            alternative_path_in_target = target_dir / suggested_filename
            if alternative_path_in_target.is_file() and alternative_path_in_target.stat().st_size > 0:
                print(f"DEBUG [C&W]: File found at {alternative_path_in_target} (likely Playwright default save). Using this.")
                save_path = alternative_path_in_target
            else:
                files_present = [f.name for f in target_dir.iterdir() if f.is_file()]
                print(f"DEBUG [C&W]: Files currently in {target_dir}: {files_present}")
                return ActionResult(error=f"File '{suggested_filename}' not properly saved to '{save_path}'. It might be in Playwright's temp cache: '{temp_download_path}' or download failed.")
        
        print(f"DEBUG [C&W]: File confirmed at {save_path}, size: {save_path.stat().st_size}")

        file_info_payload = {
            "actual_downloaded_filename": save_path.name,
            "download_path": str(save_path.resolve()),
            "size_bytes": save_path.stat().st_size,
            "modified_time_utc": datetime.fromtimestamp(save_path.stat().st_mtime, tz=timezone.utc).isoformat(),
            "status": "download_successful_and_saved"
        }
        return ActionResult(
            extracted_content=f"Successfully downloaded and saved '{save_path.name}' to '{save_path.parent}'.",
            payload=file_info_payload,
            include_in_memory=True
        )

    except Exception as e:
        print(f"ERROR CAUGHT in click_and_wait_for_download's download part: {type(e).__name__}: {str(e)}")
        return ActionResult(error=f"Error during click and download for element index {index}: {type(e).__name__} - {str(e)}")

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
    p.add_argument(
        "--target-download-dir",
        type=str,
        required=True, # The Go subprocess call provides a default if user doesn't specify in YAML workflow
        help="Absolute path to the directory where browser downloads should be saved.",
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

    browser_profile_args = {"user_data_dir": None}
    if args.target_download_dir:
        # Ensure the path passed to BrowserProfile is absolute, Playwright prefers this.
        # Go side should already make it absolute, but a check here doesn't hurt.
        abs_target_download_dir = os.path.abspath(args.target_download_dir)
        browser_profile_args["downloads_dir"] = abs_target_download_dir
        print(f"DEBUG: Configuring BrowserProfile with downloads_dir: {abs_target_download_dir}")
    else:
        print("CRITICAL ERROR: --target-download-dir was not provided by orchestrator! Exiting.")
        os._exit(1) # Exit if this critical setup info is missing

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
    upload_decorator = controller.action(
        "Uploads a file from the host system to a file input element on the current webpage. Requires the index of the file input element and the path of the file (from available_file_paths) to upload."
    )(upload_file_action)

    get_download_info_decorator = controller.action(
        "Retrieves information about the most recently downloaded file found in the target download directory."
    )(get_last_downloaded_file_info)

    click_and_download_decorator = controller.action(
        "Clicks an element (by index) expected to trigger a file download and waits for the download to complete."
    )(click_and_wait_for_download)

    llm_instance = ChatOpenAI(
        model="gpt-4o",
        temperature=0.3,
        api_key=os.getenv("OPENAI_API_KEY"),
    )

    # Consider making headless configurable via an arg 
    browser_session = BrowserSession(
        headless=False, 
        browser_profile=BrowserProfile(**browser_profile_args)
    ) 

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
        try:
            await browser_session.close()
        except Exception:
            pass
        print("Done!")

if __name__ == "__main__":
    asyncio.run(main())