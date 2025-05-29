from browser_use import Agent, BrowserSession
from browser_use.agent.views import ActionResult
from browser_use.controller.service import Controller
from langchain_openai import ChatOpenAI
from pydantic import BaseModel
from dotenv import load_dotenv
import asyncio
import argparse
import os


class Summary(BaseModel):
    summary: str


controller = Controller(output_model=Summary)


@controller.action("Upload file to interactive element with file path ")
async def upload_file(
    index: int,
    path: str,
    browser_session: BrowserSession,
    available_file_paths: list[str],
):
    if path not in available_file_paths:
        return ActionResult(error=f"File path {path} is not in the allowed list")
    el_info = await browser_session.find_file_upload_element_by_index(index)
    if not el_info:
        return ActionResult(error=f"No file-upload element at index {index}")
    handle = await browser_session.get_locate_element(el_info)
    try:
        await handle.set_input_files(path)
        return ActionResult(extracted_content=f"Uploaded “{path}” at index {index}")
    except Exception as e:
        return ActionResult(error=str(e))


def parse_args():
    p = argparse.ArgumentParser()
    p.add_argument("--prompt", required=True, help="The LLM task prompt")
    p.add_argument(
        "--out", default="output/result.json", help="Where to write the JSON result"
    )
    p.add_argument(
        "--upload-file-paths",  # New argument
        nargs="*",  # Accepts zero or more file paths
        default=[],
        help="List of local file paths available for upload by the agent for this task",
    )
    return p.parse_args()


async def main():
    args = parse_args()
    load_dotenv()

    model = ChatOpenAI(
        model="gpt-4o",
        temperature=0.3,
        api_key=os.getenv("OPENAI_API_KEY"),
    )

    browser_session = BrowserSession(
        headless=False, user_data_dir=None  # temporary profile for clean headless run
    )

    agent = Agent(
        task=args.prompt,
        llm=model,
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
        print(e)
    finally:
        print("Done!")


if __name__ == "__main__":
    asyncio.run(main())
