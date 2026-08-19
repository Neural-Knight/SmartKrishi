import json, logging
import os
from google import genai
from google.genai import types
from ..state   import State
from ..history import search as hist_search
from ..tools   import TOOLS
from ..file_manager import get_chat_files
from ..client_manager import client_manager

MODEL  = "gemini-2.5-pro"
SEARCH = types.Tool(google_search=types.GoogleSearch())
log    = logging.getLogger("MainAgent")

def main_agent_node(state: State) -> State:
    # History is already loaded in state from chat context
    # No need to search again since we have chat-specific history

    # call declared tools
    for name in state.plan.get("tools_needed", []):
        fn = TOOLS.get(name)
        if fn:
            if name == "chat_history":
                # Special handling for chat history tool
                state.tool_calls[name] = fn(
                    query=state.plan.get("location", ""),
                    user_id=state.user_id,
                    chat_id=state.chat_id,
                    limit=10
                )
            else:
                state.tool_calls[name] = fn(state.plan.get("location", ""))

    hist = "\n".join(f"{m['role']}: {m['msg']}" for m in state.history)
    
    # Get uploaded files for this chat to include in context
    files_context = ""
    gemini_files = []  # Store actual Gemini file references for code execution
    try:
        chat_files = get_chat_files(state.chat_id, state.user_id)
        if chat_files:
            files_context = "\n\nUploaded Files Available:\n"
            for file_info in chat_files:
                files_context += f"- File: {file_info['original_filename']} ({file_info['file_type']})\n"
                if file_info['summary']:
                    files_context += f"  Summary: {file_info['summary']}\n"
                files_context += f"  Status: {file_info['processing_status']}\n"
                
                # If file has Gemini file ID, we can reference it for code execution
                if file_info.get('gemini_file_id'):
                    try:
                        # Use the client manager to get the file reference
                        gemini_file = client_manager.get_file(state.chat_id, file_info['gemini_file_id'])
                        if gemini_file:
                            # Create proper Part object with fileData for code execution
                            file_part = types.Part(file_data=types.FileData(file_uri=gemini_file.uri))
                            gemini_files.append(file_part)
                            files_context += f"  Available for code execution: Yes (URI: {gemini_file.uri})\n"
                        else:
                            files_context += f"  Available for code execution: No (file may have expired)\n"
                    except Exception as e:
                        log.warning(f"Failed to get Gemini file reference {file_info['gemini_file_id']}: {e}")
                        files_context += f"  Available for code execution: No\n"
                
            files_context += "\nNote: Files with code execution support can be analyzed directly with Python code.\n"
    except Exception as e:
        log.warning("Failed to get file context: %s", e)
        files_context = ""
    
    prompt = (f"User: {state.user_query}\n\nHistory:\n{hist}\n\n"
              f"Tool data:\n{json.dumps(state.tool_calls, indent=2)}\n"
              f"{files_context}\n"
              "Give 1) Executive Summary 2) Detailed Analysis 3) "
              "Specific Recommendations 4) Risks 5) Timeline.")
    
    # attempt model call, fallback on missing API key or errors
    try:
        # Prepare content including files and prompt
        contents = []
        if gemini_files:
            # Add uploaded files to the conversation
            contents.extend(gemini_files)
        contents.append(prompt)
        
        # Configure tools - include code execution if we have data files
        tools = [SEARCH]
        if gemini_files:
            # Add code execution tool if we have data files (CSV/XLSX) or need advanced analysis
            has_data_files = False
            has_images = False
            
            for file_part in gemini_files:
                if hasattr(file_part, 'file_data') and file_part.file_data and file_part.file_data.file_uri:
                    uri = file_part.file_data.file_uri
                    if uri.endswith(('.csv', '.xlsx')):
                        has_data_files = True
                    elif 'image' in uri or uri.endswith(('.jpg', '.jpeg', '.png', '.gif')):
                        has_images = True
            
            # Add code execution for data analysis or advanced image processing
            if has_data_files or has_images:
                tools.append(types.Tool(code_execution=types.ToolCodeExecution()))
        
        # Use the client manager to ensure consistent file access
        rsp = client_manager.generate_content(
            chat_id=state.chat_id,
            model=MODEL,
            contents=contents,
            config=types.GenerateContentConfig(tools=tools)
        )
        
        state.draft_answer = rsp.text if rsp and hasattr(rsp, 'text') else ""
    except Exception as e:
        log.warning("MainAgent model call failed: %s", e)
        # leave draft_answer blank or provide a fallback message
        state.draft_answer = ""
    return state
