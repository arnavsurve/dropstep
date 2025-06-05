import os
import tempfile
from typing import Optional
from dotenv import load_dotenv
from browser_use import BrowserProfile
from pathlib import Path

def load_environment():
    load_dotenv()

def get_openai_api_key():
    key = os.getenv("OPENAI_API_KEY")
    if not key:
        print("Warning: OPENAI_API_KEY not found in environment.")
    return key

def create_browser_profile(target_download_dir_str: Optional[str], user_data_dir: Optional[str] = None) -> BrowserProfile:
    profile_args = {"user_data_dir": user_data_dir} # For incognito-like session
    if target_download_dir_str:
        abs_target_download_dir = os.path.abspath(target_download_dir_str)
        profile_args["downloads_dir"] = abs_target_download_dir
    else:
        # This case should be handled by the caller (main_agent.py) ensuring a dir is always passed
        print("CRITICAL ERROR: target_download_dir not provided for BrowserProfile creation!")
        # Fallback to a temporary default to avoid crashing BrowserProfile init
        dummy_dir = Path(tempfile.gettempdir()) / "dropstep_agent_dummy_downloads_fallback"
        dummy_dir.mkdir(parents=True, exist_ok=True)
        profile_args["downloads_dir"] = str(dummy_dir)

    return BrowserProfile(**profile_args)