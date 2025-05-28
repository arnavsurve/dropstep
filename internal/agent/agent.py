from browser_use import Agent, BrowserSession, BrowserProfile
from browser_use.controller.service import Controller
from langchain_openai import ChatOpenAI
from pydantic import BaseModel
from dotenv import load_dotenv
import asyncio
import argparse
import os


class Summary(BaseModel):
    summary: str


def parse_args():
    p = argparse.ArgumentParser()
    p.add_argument("--task", required=True, help="The LLM task prompt")
    p.add_argument(
        "--out", default="output/result.json", help="Where to write the JSON result"
    )
    return p.parse_args()


async def main():
    args = parse_args()
    load_dotenv()

    controller = Controller(output_model=Summary)

    model = ChatOpenAI(
        model="gpt-4o",
        temperature=0.3,
        api_key=os.getenv("OPENAI_API_KEY"),
    )

    browser_session = BrowserSession(
        headless=True, user_data_dir=None  # temporary profile for clean headless run
    )

    agent = Agent(
        task=args.task,
        llm=model,
        controller=controller,
        browser_session=browser_session,
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
