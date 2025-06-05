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
    if not browser_session.browser_profile or not browser_session.browser_profile.downloads_dir:
        return ActionResult(error="Browser download directory not configured in session profile.")
    target_dir = Path(browser_session.browser_profile.downloads_dir)
    
    if not target_dir.exists():
        try:
            target_dir.mkdir(parents=True, exist_ok=True)
        except Exception as e_mkdir:
            return ActionResult(error=f"Target download directory '{target_dir}' does not exist and could not be created: {e_mkdir}")

    page = await browser_session.get_current_page()
    if not page:
        return ActionResult(error="No active page found in browser session.")

    element_handle = None
    el_info = None # Define el_info here so it's in scope for the print after try block
    try:
        current_state_summary = await browser_session.get_state_summary(cache_clickable_elements_hashes=True)
        
        if not current_state_summary or not current_state_summary.selector_map:
            return ActionResult(error="Failed to get page state or no interactive elements found.")

        el_info = current_state_summary.selector_map.get(index) # el_info is DOMElementNode
        
        debug_label = "N/A"
        debug_role = "N/A"
        debug_tag = "N/A"
        if el_info:
            if hasattr(el_info, 'attributes') and isinstance(el_info.attributes, dict):
                debug_label = el_info.attributes.get('aria-label', debug_label)
                debug_role = el_info.attributes.get('role', debug_role)
            if hasattr(el_info, 'tag_name'): # DOMElementNode has tag_name
                debug_tag = el_info.tag_name
            elif hasattr(el_info, 'html_tag'): # Fallback if structure changes
                debug_tag = el_info.html_tag
        
        if not el_info: # Check el_info after trying to populate debug strings
            return ActionResult(error=f"Element with index {index} not found in current page's interactive elements.")
        
        element_handle = await browser_session.get_locate_element(el_info) # This should accept DOMElementNode
        if not element_handle:
            return ActionResult(error=f"Could not get Playwright handle for element index {index} (tag: {debug_tag}, label: {debug_label}).")

    except Exception as e_find: # Catch any other unexpected error during element finding
        return ActionResult(error=f"Error finding/locating element at index {index}: {str(e_find)}")

    # --- Perform click and wait for download ---
    try:
        async with page.expect_download(timeout=30000) as download_info_ctx:
            click_target_text = "N/A"
            try:
                click_target_text = await element_handle.text_content(timeout=1000)
            except:
                pass # Ignore if text_content fails, not critical for click
            await element_handle.click(timeout=10000) 
        
        download = await download_info_ctx.value 

        suggested_filename = download.suggested_filename
        save_path = target_dir / suggested_filename
        
        await download.save_as(save_path)

        if not save_path.is_file() or save_path.stat().st_size == 0:
            alternative_path_in_target = target_dir / suggested_filename
            if alternative_path_in_target.is_file() and alternative_path_in_target.stat().st_size > 0:
                save_path = alternative_path_in_target
            else:
                temp_download_path_str = "unknown Playwright temp location"
                try:
                    temp_download_path_str = str(await download.path())
                except Exception:
                    pass # Ignore if path() fails here
                return ActionResult(error=f"File '{suggested_filename}' not properly saved to '{save_path}'. It might be in Playwright's temp cache: '{temp_download_path_str}' or download failed.")
        
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
        return ActionResult(error=f"Error during click and download for element index {index}: {type(e).__name__} - {str(e)}")