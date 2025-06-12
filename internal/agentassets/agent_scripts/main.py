import asyncio
import json
import os
from pathlib import Path
from typing import Type

import cli
import models
import actions
import settings

from langchain_openai import ChatOpenAI
from pydantic import BaseModel as PydanticBaseModel
from browser_use import Agent, BrowserSession, Controller, BrowserProfile

async def run_agent_logic():
    args = cli.parse_agent_args()

    print(f"PYTHON MAIN: args.target_download_dir received from CLI: {args.target_download_dir}")
    browser_profile_obj = settings.create_browser_profile(args.target_download_dir, user_data_dir=None)

    settings.load_environment()

    output_model_class: Type[PydanticBaseModel] = models.Summary
    if args.output_schema:
        try:
            json.loads(args.output_schema)
            output_model_class = models.get_pydantic_model_from_schema(
                args.output_schema, args.model_name
            )
            if not output_model_class: # get_pydantic_model_from_schema returns Summary on failure
                output_model_class = models.Summary
            print(f"Using output model: {output_model_class.__name__}")
        except json.JSONDecodeError as e:
            print(f"Error: --output-schema is not valid JSON: {e}. Defaulting to Summary.")
            output_model_class = models.Summary
        except Exception as e:
            print(f"Error processing output schema: {e}. Defaulting to Summary.")
            output_model_class = models.Summary
    
    controller = Controller(output_model=output_model_class)

    controller.action(
        "Uploads a file from the host system..."
    )(actions.upload_file_action_impl)
    controller.action(
        "Retrieves information about the most recently downloaded file..."
    )(actions.get_last_downloaded_file_info_impl)
    controller.action(
        "Clicks an element (by index) expected to trigger a file download and waits..."
    )(actions.click_and_wait_for_download_impl)

    api_key = settings.get_openai_api_key()
    llm_instance = ChatOpenAI(
        model="gpt-4o",
        temperature=0.3,
        api_key=api_key,
    )

    browser_session_args = {
        "headless": False,
        "browser_profile": browser_profile_obj,
    }

    if args.allowed_domains:
        browser_session_args["allowed_domains"] = args.allowed_domains
        print(f"Restricting navigation to domains: {args.allowed_domains}")

    browser_session = BrowserSession(**browser_session_args) 

    agent = Agent(
        task=args.prompt,
        llm=llm_instance,
        controller=controller,
        browser_session=browser_session,
        available_file_paths=args.upload_file_paths,
    )

    result_json_str = None
    try:
        history = await agent.run()
        result_json_str = history.final_result()

        if result_json_str is not None:
            output_file_path = Path(args.out)
            output_file_path.parent.mkdir(parents=True, exist_ok=True)
            with open(output_file_path, "w", encoding="utf-8") as f:
                f.write(result_json_str)
                f.flush()
                try:
                    os.fsync(f.fileno())
                except OSError as e_fsync:
                    print(f"Warning: os.fsync error on {output_file_path}: {e_fsync}")
            print(f"Wrote result to {args.out}")
        else:
            print(f"ERROR: Agent did not produce a final JSON result. Output to {args.out} skipped.")
    except Exception as e:
        print(f"Error during agent run: {type(e).__name__}: {e}")
    finally:
        print("Done!")
        try:
            if browser_session and hasattr(browser_session, 'stop'):
                print("Attempting to stop browser session...")
                await browser_session.stop()
                print("Browser session stopped.")
        except AttributeError:
            print("Browser session object or stop method not as expected or already closed.")
        except Exception as e_stop:
            print(f"Error stopping browser session: {e_stop}")

if __name__ == "__main__":
    asyncio.run(run_agent_logic())