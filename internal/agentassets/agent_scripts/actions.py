import os
from pathlib import Path
from datetime import datetime, timezone
from browser_use import BrowserSession, ActionResult

async def upload_file_action_impl(
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
        return ActionResult(error=f"No file-upload element found at index {index}")
    handle = await browser_session.get_locate_element(el_info)
    try:
        await handle.set_input_files(path)
        return ActionResult(extracted_content=f"Successfully uploaded file “{path}” to element at index {index}.", include_in_memory=True)
    except Exception as e:
        return ActionResult(error=f"Failed to upload file “{path}” at index {index}: {str(e)}")

async def get_last_downloaded_file_info_impl(
    browser_session: BrowserSession,
) -> ActionResult:
    if not browser_session.browser_profile or not browser_session.browser_profile.downloads_dir:
        return ActionResult(error="Browser download directory not configured in session profile.")
    target_dir = Path(browser_session.browser_profile.downloads_dir)
    if not target_dir.exists():
        return ActionResult(error=f"Target download directory '{target_dir}' does not exist.")
    files_in_dir = [f for f in target_dir.iterdir() if f.is_file()]
    if not files_in_dir:
        return ActionResult(extracted_content="No files found in the target download directory.", include_in_memory=True)
    try:
        latest_file = max(files_in_dir, key=lambda f: f.stat().st_mtime)
    except ValueError:
         return ActionResult(extracted_content="No files to determine latest in target download directory.", include_in_memory=True)
    file_info_payload = {
        "actual_downloaded_filename": latest_file.name,
        "download_path": str(latest_file.resolve()),
        "size_bytes": latest_file.stat().st_size,
        "modified_time_utc": datetime.fromtimestamp(latest_file.stat().st_mtime, tz=timezone.utc).isoformat(),
        "status": "confirmed_in_target_dir"
    }
    return ActionResult(
        extracted_content=f"Confirmed latest file in target download directory: {latest_file.name}",
        payload=file_info_payload, 
        include_in_memory=True
    )


async def click_and_wait_for_download_impl(
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