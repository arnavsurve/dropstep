import argparse


def parse_agent_args():
    p = argparse.ArgumentParser(description="Dropstep Browser Agent")
    p.add_argument("--prompt", required=True, help="The LLM task prompt")
    p.add_argument(
        "--out",
        default=".dropstep/output/result.json",
        help="Where to write the JSON result",
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
        required=True,
        help="Absolute path to the directory where browser downloads should be saved.",
    )
    p.add_argument(
        "--allowed-domains",
        nargs="*",
        default=[],
        help="List of allowed domains that the agent can navigate to.",
    )
    p.add_argument(
        "--max-steps",
        type=int,
        default=100,
        help="Maximum number of steps an agent can take before failing the execution run.",
    )
    p.add_argument(
        "--max-failures",
        type=int,
        default=3,
        help="Maximum number of failures an agent can incur before failing the execution run.",
    )
    p.add_argument(
        "--data-dir",
        type=str,
        default=None,
        help="Browser data directory to use for the agent's browser.",
    )
    return p.parse_args()

