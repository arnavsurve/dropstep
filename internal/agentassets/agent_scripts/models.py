import json
import os
import sys
import tempfile
import importlib.util
from pathlib import Path
from typing import Type, Optional
from pydantic import BaseModel as PydanticBaseModel

from datamodel_code_generator import (
    generate,
    InputFileType,
    DataModelType,
)

class Summary(PydanticBaseModel):
    summary: str

def get_pydantic_model_from_schema(
    json_schema_str: str,
    model_name: str = "DynamicOutputModel"
) -> Type[PydanticBaseModel]:
    with tempfile.NamedTemporaryFile(mode="w+", suffix=".py", delete=False) as tmp_model_file:
        output_path = Path(tmp_model_file.name)
        generated_model_class: Optional[Type[PydanticBaseModel]] = None
        temp_dir = str(output_path.parent)
        module_name_for_import = f"temp_dyn_model_{output_path.stem}"

        try:
            generate(
                json_schema_str,
                input_file_type=InputFileType.JsonSchema,
                input_filename="schema.json",
                output=output_path,
                output_model_type=DataModelType.PydanticV2BaseModel,
                class_name=model_name,
            )

            if temp_dir not in sys.path:
                sys.path.insert(0, temp_dir)

            spec = importlib.util.spec_from_file_location(module_name_for_import, str(output_path))
            if not spec or not spec.loader:
                raise ImportError(f"Could not create module spec for {output_path}")

            generated_module = importlib.util.module_from_spec(spec)
            sys.modules[module_name_for_import] = generated_module
            spec.loader.exec_module(generated_module)

            model_class_candidate = None
            if hasattr(generated_module, model_name) and \
               isinstance(getattr(generated_module, model_name), type) and \
               issubclass(getattr(generated_module, model_name), PydanticBaseModel):
                model_class_candidate = getattr(generated_module, model_name)
            else:
                for name, obj in vars(generated_module).items():
                    if isinstance(obj, type) and \
                       issubclass(obj, PydanticBaseModel) and \
                       obj is not PydanticBaseModel and obj is not Summary:
                        model_class_candidate = obj
                        break
            
            if model_class_candidate:
                model_class_candidate.model_rebuild(force=True)
                generated_model_class = model_class_candidate
            else:
                print(f"Error: Could not load Pydantic model from {module_name_for_import}.")
        except Exception as e:
            print(f"ERROR CAUGHT in get_pydantic_model_from_schema: {type(e).__name__}: {e}")
            if output_path.exists():
                print(f"Generated code (if any) at: {output_path}")
        finally:
            if module_name_for_import in sys.modules:
                del sys.modules[module_name_for_import]
            if temp_dir in sys.path:
                sys.path.remove(temp_dir)
            if output_path.exists():
                pyc_file = output_path.with_suffix('.pyc')
                if pyc_file.exists():
                    try: os.unlink(pyc_file)
                    except: pass
                try: os.unlink(output_path)
                except Exception as e_unlink: print(f"Warning: Could not delete {output_path}: {e_unlink}")
        
        return generated_model_class if generated_model_class else Summary