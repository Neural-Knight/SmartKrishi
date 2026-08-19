import os, logging
from uuid import uuid4
from typing import Optional
from fastapi import FastAPI, UploadFile, File, Form
from fastapi.middleware.cors import CORSMiddleware
import os
from dotenv import load_dotenv
# load .env file for API keys
load_dotenv(os.path.join(os.path.dirname(__file__), '..', '.env'))
from fastapi.responses import JSONResponse  # type: ignore
import os
from datetime import datetime
import time

from .state   import State
from .nodes.planner    import planner_node
from .nodes.main_agent import main_agent_node
from .nodes.checker    import checker_node
from .history import add_message, get_chat_messages, create_chat, get_user_chats, get_chat_info, delete_chat
from .media   import save_file
from .nodes.main_agent import MODEL as AGENT_MODEL, SEARCH
from .nodes.planner    import MODEL as PLANNER_MODEL
from .tools            import TOOLS
from fastapi.responses import StreamingResponse
from fastapi.responses import FileResponse
import json
from google import genai
from fastapi.responses import JSONResponse
from google.genai import types

# in-memory store for user tool preferences
USER_TOOL_PREFS: dict[str, list[str]] = {}

api = FastAPI(title="SmartKrishi (multi-tenant)")

# Add CORS middleware
api.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # In production, replace with specific origins
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

log = logging.getLogger("API")

def _anon() -> str:
    return uuid4().hex

def get_current_datetime_info() -> str:
    """Get current date, time and timezone information"""
    try:
        # Get current local time
        now = datetime.now()
        
        # Get timezone information
        timezone_name = time.tzname[0] if time.tzname else "Local"
        
        # Get UTC time
        utc_now = datetime.utcnow()
        
        # Calculate timezone offset
        timezone_offset = time.timezone / 3600  # Convert seconds to hours
        offset_sign = "+" if timezone_offset <= 0 else "-"
        offset_hours = abs(int(timezone_offset))
        offset_minutes = abs(int((timezone_offset % 1) * 60))
        
        # Format the datetime information
        datetime_info = f"""
Current Date & Time Information:
- Local Time: {now.strftime('%Y-%m-%d %H:%M:%S')} ({timezone_name})
- UTC Time: {utc_now.strftime('%Y-%m-%d %H:%M:%S')} UTC
- Timezone Offset: UTC{offset_sign}{offset_hours:02d}:{offset_minutes:02d}
- Day of Week: {now.strftime('%A')}
- Date: {now.strftime('%B %d, %Y')}
- Time: {now.strftime('%I:%M %p')}
- Season: {get_season(now.month)}
"""
        return datetime_info.strip()
    except Exception as e:
        # Fallback to basic datetime if timezone detection fails
        now = datetime.now()
        return f"""
Current Date & Time Information:
- Current Time: {now.strftime('%Y-%m-%d %H:%M:%S')}
- Day of Week: {now.strftime('%A')}
- Date: {now.strftime('%B %d, %Y')}
- Time: {now.strftime('%I:%M %p')}
- Note: Advanced timezone detection failed: {str(e)}
"""

def get_season(month: int) -> str:
    """Get current season based on month (Northern Hemisphere)"""
    if month in [12, 1, 2]:
        return "Winter"
    elif month in [3, 4, 5]:
        return "Spring"
    elif month in [6, 7, 8]:
        return "Summer"
    elif month in [9, 10, 11]:
        return "Autumn/Fall"
    else:
        return "Unknown"

# Frontend route
@api.get("/")
async def serve_frontend():
    """Serve the frontend HTML file"""
    return FileResponse("frontend.html")

# Optional enhanced UI route
@api.get("/enhanced")
async def serve_frontend_enhanced():
    """Serve the enhanced frontend with live code execution panels"""
    return FileResponse("frontend_enhanced.html")

# Chat Management Endpoints
@api.post("/users/{user_id}/chats")
async def create_user_chat(user_id: str, chat_name: str = Form(...)):
    """Create a new chat for a user"""
    chat_id = create_chat(user_id, chat_name)
    return {"chat_id": chat_id, "user_id": user_id, "chat_name": chat_name}

@api.get("/users/{user_id}/chats")
async def list_user_chats(user_id: str):
    """Get all chats for a user"""
    chats = get_user_chats(user_id)
    return {"user_id": user_id, "chats": chats}

@api.get("/users/{user_id}/chats/{chat_id}")
async def get_chat_details(user_id: str, chat_id: str):
    """Get specific chat details"""
    chat = get_chat_info(chat_id, user_id)
    if not chat:
        return JSONResponse({"error": "Chat not found"}, status_code=404)
    return {"user_id": user_id, **chat}

@api.delete("/users/{user_id}/chats/{chat_id}")
async def delete_user_chat(user_id: str, chat_id: str):
    """Delete a chat and all its messages"""
    success = delete_chat(chat_id, user_id)
    if not success:
        return JSONResponse({"error": "Chat not found"}, status_code=404)
    return {"message": "Chat deleted successfully", "chat_id": chat_id}

@api.put("/users/{user_id}/chats/{chat_id}")
async def rename_user_chat(user_id: str, chat_id: str, new_name: str = Form(...)):
    """Rename a chat for a user"""
    from .history import rename_chat
    success = rename_chat(chat_id, user_id, new_name)
    if not success:
        return JSONResponse({"error": "Chat not found"}, status_code=404)
    return {"message": "Chat renamed successfully", "chat_id": chat_id, "chat_name": new_name}

@api.get("/users/{user_id}/chats/{chat_id}/messages")
async def get_chat_history(user_id: str, chat_id: str, limit: int = 100):
    """Get messages for a specific chat"""
    # Verify chat ownership
    chat = get_chat_info(chat_id, user_id)
    if not chat:
        return JSONResponse({"error": "Chat not found"}, status_code=404)
    
    messages = get_chat_messages(chat_id, user_id, limit)
    return {"user_id": user_id, "chat_id": chat_id, "messages": messages}

@api.post("/users/{user_id}/chats/{chat_id}/messages")
async def send_message_to_chat(
    user_id: str,
    chat_id: str,
    message: str = Form(...),
    model: str = Form("gemini-2.5-flash"),
    tools: str = Form(None),  # Comma-separated tool names
    logs: bool = Form(False)
):
    """Send a message to a specific chat and get streaming response"""
    
    # Parse tools if provided  
    include_tools = None
    if tools:
        include_tools = [t.strip() for t in tools.split(',') if t.strip()]
    
    # Use the existing ask_stream logic
    return await ask_stream(
        q=message,
        user_id=user_id,
        chat_id=chat_id,
        include_tools=','.join(include_tools) if include_tools else None,
        logs=logs
    )

@api.post("/ask")
async def ask(q: str = Form(...), 
              user_id: str = Form(default_factory=_anon),
              chat_id: str = Form(...),
              logs: bool = Form(False)):
    """Ask a question in a specific chat with optional detailed logging"""
    # Verify chat exists and belongs to user
    chat = get_chat_info(chat_id, user_id)
    if not chat:
        return JSONResponse({"error": "Chat not found"}, status_code=404)
    
    # record user query
    add_message(chat_id, user_id, "user", q)
    
    # Get chat history for context
    history = get_chat_messages(chat_id, user_id, limit=20)
    
    # initialize state
    state = State(user_id=user_id, chat_id=chat_id, user_query=q, history=history)
    
    # Collect detailed logs if requested
    detailed_logs = {
        "planner": {},
        "tool_calls": {},
        "main_agent": {},
        "checker": {},
        "grounding": {}
    } if logs else None
    
    # planner stage
    if logs and detailed_logs:
        detailed_logs["planner"]["input"] = {"user_query": q, "history": [msg for msg in history[-5:]]}  # Last 5 messages
    state = planner_node(state)
    if logs and detailed_logs:
        detailed_logs["planner"]["output"] = state.plan
    
    # tool calls and agent
    if logs and detailed_logs:
        detailed_logs["tool_calls"]["pre_execution"] = state.plan.get("tools_needed", [])
    state = main_agent_node(state)
    if logs and detailed_logs:
        detailed_logs["tool_calls"]["results"] = state.tool_calls
        detailed_logs["main_agent"]["input"] = {
            "history": [msg for msg in state.history[-5:]],
            "tool_calls": state.tool_calls,
            "user_query": state.user_query
        }
        detailed_logs["main_agent"]["pre_checker"] = {
            "draft_answer": state.draft_answer[:200] + "..." if len(state.draft_answer) > 200 else state.draft_answer
        }
    
    # final approval
    state = checker_node(state)
    if logs and detailed_logs:
        detailed_logs["checker"]["output"] = {
            "approved": state.approved,
            "confidence": state.confidence,
            "issues": state.issues
        }
    
    # Get grounding information from the main agent call
    grounding_info = {}
    if logs:
        try:
            # Make a non-streaming call to get grounding metadata
            client = genai.Client()
            hist = "\n".join(f"{m['role']}: {m['msg']}" for m in state.history)
            datetime_context = get_current_datetime_info()
            prompt_agent = (
                f"You are SmartKrishi Agent, an advanced AI agricultural advisor specializing in farming intelligence, crop management, and agricultural technology.\n\n"
                f"{datetime_context}\n\n"
                f"LANGUAGE INSTRUCTION: Respond in the EXACT same language and script as the user's query. If the user writes in native script (like Hindi Devanagari, Bengali, Tamil, etc.), respond in that exact script. If the user writes in English script but in a different language (like Hindi in English letters), respond in the same format. Match the user's linguistic style completely.\n\n"
                f"CODE EXECUTION OUTPUT INSTRUCTION: Don't just mention that you ran code - show the user what the code produced by writing the output yourself in the answer as text/markdown. That is copy the output and write it fully again as a normal response.\n\n"
                f"THINKING GUIDE:\n"
                f"1. Identify and clearly define the core agricultural problem or question\n"
                f"2. Reflect on whether the problem can be fully understood or if any assumptions need to be made. If assumptions are necessary, state them transparently. If there are any ambiguities or incomplete information, ask the user for clarification and stop the response\n"
                f"3. Once ambiguities are clear, break the problem down into smaller, logical parts or sub-questions, and solve them systematically\n"
                f"4. Search the internet and write code when required for data analysis or agricultural calculations\n"
                f"5. Before delivering your final answer, reflect on the steps taken. Ask yourself if the approach was thorough, if all aspects were addressed, and if there are any gaps in reasoning\n"
                f"6. Provide the comprehensive agricultural solution with clear explanations of why it's the best approach based on your analysis\n\n"
                f"User Query: {state.user_query}\n\n"
                f"Conversation History:\n{hist}\n\n"
                f"Available Agricultural Data & Tools:\n{json.dumps(state.tool_calls, indent=2)}\n\n"
                f"As SmartKrishi Agent, provide expert agricultural guidance following the thinking guide above."
            )
            
            config = types.GenerateContentConfig(
                thinking_config=types.ThinkingConfig(include_thoughts=True),
                tools=[SEARCH]
            )
            
            response = client.models.generate_content(
                model=AGENT_MODEL,
                contents=prompt_agent,
                config=config,
            )
            
            # Extract grounding metadata
            if response.candidates and len(response.candidates) > 0:
                candidate = response.candidates[0]
                if hasattr(candidate, 'grounding_metadata') and candidate.grounding_metadata:
                    grounding_metadata = candidate.grounding_metadata
                    
                    if hasattr(grounding_metadata, 'web_search_queries') and grounding_metadata.web_search_queries:
                        grounding_info["web_search_queries"] = list(grounding_metadata.web_search_queries)
                    
                    if hasattr(grounding_metadata, 'grounding_chunks') and grounding_metadata.grounding_chunks:
                        chunks = []
                        for chunk_data in grounding_metadata.grounding_chunks:
                            if hasattr(chunk_data, 'web') and chunk_data.web:
                                chunks.append({
                                    "uri": chunk_data.web.uri,
                                    "title": chunk_data.web.title if hasattr(chunk_data.web, 'title') else "Unknown"
                                })
                        grounding_info["grounding_chunks"] = chunks
                    
                    if hasattr(grounding_metadata, 'grounding_supports') and grounding_metadata.grounding_supports:
                        supports = []
                        for support in grounding_metadata.grounding_supports:
                            if hasattr(support, 'segment') and support.segment and hasattr(support, 'grounding_chunk_indices') and support.grounding_chunk_indices:
                                supports.append({
                                    "segment": {
                                        "start_index": support.segment.start_index if hasattr(support.segment, 'start_index') else 0,
                                        "end_index": support.segment.end_index if hasattr(support.segment, 'end_index') else 0,
                                        "text": support.segment.text if hasattr(support.segment, 'text') else ""
                                    },
                                    "grounding_chunk_indices": list(support.grounding_chunk_indices)
                                })
                        grounding_info["grounding_supports"] = supports
                        
            if logs and detailed_logs:
                detailed_logs["grounding"] = grounding_info
        except Exception as e:
            if logs and detailed_logs:
                detailed_logs["grounding"]["error"] = str(e)
    
    # record assistant response
    add_message(chat_id, user_id, "assistant", state.draft_answer)
    
    response_data = {
        "answer": state.draft_answer,
        "approved": state.approved,
        "confidence": round(state.confidence, 2),
        "issues": state.issues,
        "tools": list(state.tool_calls.keys()),
        "user_id": user_id,
        "chat_id": chat_id
    }
    
    if logs:
        response_data["detailed_logs"] = detailed_logs
        response_data["grounding"] = grounding_info
    
    return JSONResponse(response_data)

@api.post("/upload/pdf")
async def upload_pdf(file: UploadFile = File(...),
                     user_id: str = Form(default_factory=_anon),
                     chat_id: str = Form(...)):
    """Upload PDF with enhanced processing and metadata tracking"""
    filename = file.filename or "uploaded_file.pdf"
    
    try:
        result = save_file(user_id, await file.read(), filename, "pdf", chat_id)
        return {
            "file_id": result["file_id"],
            "stored_path": result["stored_path"],
            "user_id": user_id,
            "chat_id": chat_id,
            "filename": filename,
            "status": "uploaded",
            "processing_status": result["processing_status"],
            "message": "File uploaded successfully. Processing in background..."
        }
    except Exception as e:
        log.error(f"PDF upload failed: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Upload failed: {str(e)}"}
        )

@api.post("/upload/image")
async def upload_image(file: UploadFile = File(...),
                       user_id: str = Form(default_factory=_anon),
                       chat_id: str = Form(...)):
    """Upload image with enhanced processing and metadata tracking"""
    filename = file.filename or "uploaded_file.jpg"
    
    try:
        result = save_file(user_id, await file.read(), filename, "image", chat_id)
        return {
            "file_id": result["file_id"],
            "stored_path": result["stored_path"],
            "user_id": user_id,
            "chat_id": chat_id,
            "filename": filename,
            "status": "uploaded",
            "processing_status": result["processing_status"],
            "message": "File uploaded successfully. Processing in background..."
        }
    except Exception as e:
        log.error(f"Image upload failed: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Upload failed: {str(e)}"}
        )

# File Management Endpoints
@api.post("/upload/docx")
async def upload_docx(file: UploadFile = File(...),
                      user_id: str = Form(default_factory=_anon),
                      chat_id: str = Form(...)):
    """Upload DOCX with Gemini code execution analysis"""
    filename = file.filename or "uploaded_file.docx"
    
    try:
        result = save_file(user_id, await file.read(), filename, "docx", chat_id)
        return {
            "file_id": result["file_id"],
            "stored_path": result["stored_path"],
            "user_id": user_id,
            "chat_id": chat_id,
            "filename": filename,
            "status": "uploaded",
            "processing_status": result["processing_status"],
            "message": "DOCX file uploaded successfully. Processing with code execution..."
        }
    except Exception as e:
        log.error(f"DOCX upload failed: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Upload failed: {str(e)}"}
        )

@api.post("/upload/xlsx")
async def upload_xlsx(file: UploadFile = File(...),
                      user_id: str = Form(default_factory=_anon),
                      chat_id: str = Form(...)):
    """Upload XLSX with Gemini code execution analysis and visualization"""
    filename = file.filename or "uploaded_file.xlsx"
    
    try:
        result = save_file(user_id, await file.read(), filename, "xlsx", chat_id)
        return {
            "file_id": result["file_id"],
            "stored_path": result["stored_path"],
            "user_id": user_id,
            "chat_id": chat_id,
            "filename": filename,
            "status": "uploaded",
            "processing_status": result["processing_status"],
            "message": "XLSX file uploaded successfully. Processing with data analysis..."
        }
    except Exception as e:
        log.error(f"XLSX upload failed: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Upload failed: {str(e)}"}
        )

@api.post("/upload/csv")
async def upload_csv(file: UploadFile = File(...),
                     user_id: str = Form(default_factory=_anon),
                     chat_id: str = Form(...)):
    """Upload CSV with Gemini code execution analysis and visualization"""
    filename = file.filename or "uploaded_file.csv"
    
    try:
        result = save_file(user_id, await file.read(), filename, "csv", chat_id)
        return {
            "file_id": result["file_id"],
            "stored_path": result["stored_path"],
            "user_id": user_id,
            "chat_id": chat_id,
            "filename": filename,
            "status": "uploaded",
            "processing_status": result["processing_status"],
            "message": "CSV file uploaded successfully. Processing with data analysis..."
        }
    except Exception as e:
        log.error(f"CSV upload failed: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Upload failed: {str(e)}"}
        )

@api.get("/chat/{chat_id}/files")
async def list_chat_files(user_id: str, chat_id: str):
    """List all files uploaded to a specific chat"""
    try:
        from app.file_manager import get_chat_files
        files = get_chat_files(chat_id, user_id)
        return {
            "user_id": user_id,
            "chat_id": chat_id,
            "files": files,
            "total_count": len(files)
        }
    except Exception as e:
        log.error(f"Failed to list chat files: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Failed to list files: {str(e)}"}
        )

@api.get("/file/{file_id}/details")
async def get_file_details(file_id: str, user_id: str, chat_id: str):
    """Get detailed information about a specific file including analysis"""
    try:
        from app.file_manager import get_file_metadata
        file_data = get_file_metadata(file_id, user_id)
        
        if not file_data:
            return JSONResponse(
                status_code=404,
                content={"error": "File not found"}
            )
        
        # Verify user has access to this file
        if file_data.get('user_id') != user_id or file_data.get('chat_id') != chat_id:
            return JSONResponse(
                status_code=403,
                content={"error": "Access denied"}
            )
        
        return {
            "file_id": file_id,
            "original_filename": file_data.get('original_filename'),
            "file_type": file_data.get('file_type'),
            "file_size": file_data.get('file_size'),
            "processing_status": file_data.get('processing_status'),
            "summary": file_data.get('summary'),
            "content_preview": file_data.get('content_preview'),
            "upload_date": file_data.get('created_at'),
            "gemini_file_id": file_data.get('gemini_file_ref')
        }
    except Exception as e:
        log.error(f"Failed to get file details for {file_id}: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Failed to get file details: {str(e)}"}
        )

@api.get("/users/{user_id}/files")
async def list_user_files(user_id: str, limit: int = 50):
    """List all files for a user across all chats"""
    try:
        from app.file_manager import get_user_files
        files = get_user_files(user_id, limit)
        return {
            "user_id": user_id,
            "files": files,
            "total_count": len(files)
        }
    except Exception as e:
        log.error(f"Failed to list user files: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Failed to list files: {str(e)}"}
        )

@api.get("/users/{user_id}/files/{file_id}")
async def get_file_details(user_id: str, file_id: str):
    """Get detailed information about a specific file"""
    try:
        from app.file_manager import get_file_metadata
        file_info = get_file_metadata(file_id, user_id)
        if not file_info:
            return JSONResponse(
                status_code=404,
                content={"error": "File not found"}
            )
        return file_info
    except Exception as e:
        log.error(f"Failed to get file details: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Failed to get file details: {str(e)}"}
        )

@api.delete("/users/{user_id}/files/{file_id}")
async def delete_file(user_id: str, file_id: str):
    """Delete a file and its metadata"""
    try:
        from app.file_manager import delete_file_metadata
        success = delete_file_metadata(file_id, user_id)
        if not success:
            return JSONResponse(
                status_code=404,
                content={"error": "File not found"}
            )
        return {"message": "File deleted successfully", "file_id": file_id}
    except Exception as e:
        log.error(f"Failed to delete file: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Failed to delete file: {str(e)}"}
        )

@api.get("/users/{user_id}/files/search")
async def search_user_files(user_id: str, query: str, chat_id: Optional[str] = None):
    """Search files by filename or content"""
    try:
        from app.file_manager import search_files
        files = search_files(user_id, query, chat_id)
        return {
            "user_id": user_id,
            "chat_id": chat_id,
            "query": query,
            "files": files,
            "total_matches": len(files)
        }
    except Exception as e:
        log.error(f"Failed to search files: {e}")
        return JSONResponse(
            status_code=500,
            content={"error": f"Failed to search files: {str(e)}"}
        )

@api.get("/tools")
async def list_tools():
    """List all available tool names."""
    return {"tools": list(TOOLS.keys())}
@api.get("/users/{user_id}/tools")
async def get_user_tools(user_id: str):
    """Get current tool preferences for a user."""
    prefs = USER_TOOL_PREFS.get(user_id)
    if prefs is None:
        prefs = list(TOOLS.keys())
    return {"user_id": user_id, "include_tools": prefs}
@api.post("/users/{user_id}/tools")
async def set_user_tools(user_id: str, include_tools: list[str]):
    """Set tool preferences for a user."""
    valid = [t for t in include_tools if t in TOOLS]
    USER_TOOL_PREFS[user_id] = valid
    return {"user_id": user_id, "include_tools": valid}

@api.post("/ask_stream")
async def ask_stream(
    q: str = Form(...),
    user_id: str = Form(default_factory=_anon),
    chat_id: str = Form(...),
    include_tools: str | None = Form(None),
    logs: bool = Form(False)
):
    async def event_generator():
        # Verify chat exists and belongs to user
        chat = get_chat_info(chat_id, user_id)
        if not chat:
            yield json.dumps({"type": "error", "message": "Chat not found"}) + "\n"
            return
            
        # record user query
        add_message(chat_id, user_id, "user", q)
        
        # Get chat history for context
        history = get_chat_messages(chat_id, user_id, limit=20)
        
        state = State(user_id=user_id, chat_id=chat_id, user_query=q, history=history)
        
        # Emit detailed logs if requested
        if logs:
            yield json.dumps({
                "type": "log",
                "stage": "initialization", 
                "data": {
                    "user_id": user_id,
                    "chat_id": chat_id, 
                    "query": q,
                    "history_count": len(history)
                }
            }) + "\n"
        # planner stage
        if logs:
            yield json.dumps({
                "type": "log",
                "stage": "planner_start",
                "message": "🧠 Starting planning phase..."
            }) + "\n"
            
        prompt_planner = (
            "Return JSON with keys: primary_intent, tools_needed[], location, crop.\n"
            "Query: " + state.user_query
        )
        
        if logs:
            yield json.dumps({
                "type": "log", 
                "stage": "planner_input",
                "data": {"prompt": prompt_planner}
            }) + "\n"

        # planner client (reads API key from environment)
        planner_client = genai.Client()
        chat = planner_client.chats.create(model=PLANNER_MODEL)
        rsp_planner = chat.send_message(prompt_planner)
        
        # parse planner response
        text_planner = rsp_planner.text or ""
        try:
            # Extract JSON from response that might be wrapped in markdown code blocks
            if "```json" in text_planner:
                start = text_planner.find("```json") + 7
                end = text_planner.find("```", start)
                json_text = text_planner[start:end].strip()
            else:
                json_text = text_planner.strip()
            state.plan = json.loads(json_text)
        except Exception:
            state.plan = {"primary_intent":"advise","tools_needed":["weather_api"],"location":"Unknown"}

        # Emit the plan
        yield json.dumps({
            "type": "plan",
            "plan": state.plan,
            "raw_response": text_planner
        }) + "\n"

        if logs:
            yield json.dumps({
                "type": "log",
                "stage": "planner_complete",
                "message": f"🧠 Plan created: {len(state.plan.get('tools_needed', []))} tools needed"
            }) + "\n"

        # tool calls (filterable)
        plan_tools = state.plan.get("tools_needed", [])
        if include_tools:
            selected = include_tools.split(",")
        else:
            selected = USER_TOOL_PREFS.get(user_id, list(TOOLS.keys()))
        
        if plan_tools and logs:
            yield json.dumps({
                "type": "log",
                "stage": "tools_start", 
                "message": f"🔧 Running {len(plan_tools)} tools..."
            }) + "\n"
            
        for name in plan_tools:
            if name not in selected:
                continue
            fn = TOOLS.get(name)
            if fn:
                if logs:
                    yield json.dumps({
                        "type": "log",
                        "stage": "tool_executing",
                        "message": f"🔧 Executing {name}..."
                    }) + "\n"
                    
                if name == "chat_history":
                    # Special handling for chat history tool
                    args = {
                        "query": state.plan.get("location", ""),
                        "user_id": user_id,
                        "chat_id": chat_id,
                        "limit": 10
                    }
                    result = fn(**args)
                else:
                    # Regular tools
                    args = state.plan.get("location", "")
                    result = fn(args)
                    
                state.tool_calls[name] = result
                
                # Emit tool call result
                yield json.dumps({
                    "type": "tool_call",
                    "tool": name,
                    "args": args if name != "chat_history" else "chat_history_args",
                    "result": result
                }) + "\n"

        if plan_tools and logs:
            yield json.dumps({
                "type": "log",
                "stage": "tools_complete",
                "message": f"🔧 Completed {len(state.tool_calls)} tool calls"
            }) + "\n"

        # main agent streaming
        if logs:
            yield json.dumps({
                "type": "log",
                "stage": "agent_start",
                "message": "🤔 Starting AI analysis..."
            }) + "\n"
            
        # Get file context for the agent
        files_context = ""
        gemini_files = []  # Store actual Gemini file references for code execution
        try:
            from .file_manager import get_chat_files
            chat_files = get_chat_files(chat_id, user_id)
            if logs:
                yield json.dumps({
                    "type": "log",
                    "stage": "file_check",
                    "message": f"📁 Found {len(chat_files)} files in database for chat {chat_id}"
                }) + "\n"
            
            if chat_files:
                files_context = f"\n\nUploaded Files Available ({len(chat_files)} total):\n"
                for file_info in chat_files:
                    files_context += f"- {file_info['original_filename']} ({file_info['file_type']}, {file_info['processing_status']})\n"
                    if file_info['summary']:
                        files_context += f"  Summary: {file_info['summary']}\n"
                    
                    # If file has Gemini file ID, we can reference it for code execution
                    if file_info.get('gemini_file_id'):
                        try:
                            from .client_manager import client_manager
                            # Use the client manager to get the file reference
                            gemini_file = client_manager.get_file(chat_id, file_info['gemini_file_id'])
                            if gemini_file:
                                # Create proper Part object with fileData for code execution
                                file_part = types.Part(file_data=types.FileData(file_uri=gemini_file.uri))
                                gemini_files.append(file_part)
                                files_context += f"  Available for code execution: Yes (URI: {gemini_file.uri})\n"
                            else:
                                files_context += f"  Available for code execution: No (file may have expired)\n"
                        except Exception as e:
                            files_context += f"  Available for code execution: No (error: {str(e)})\n"
                
                files_context += "\nNote: Files with code execution support can be analyzed directly with Python code.\n"
        except Exception as e:
            files_context = ""
            
        hist = "\n".join(f"{m['role']}: {m['msg']}" for m in state.history)
        datetime_context = get_current_datetime_info()
        prompt_agent = (
            f"You are SmartKrishi Agent, an advanced AI agricultural advisor specializing in farming intelligence, crop management, and agricultural technology.\n\n"
            f"{datetime_context}\n\n"
            f"LANGUAGE INSTRUCTION: Respond in the EXACT same language and script as the user's query. If the user writes in native script (like Hindi Devanagari, Bengali, Tamil, etc.), respond in that exact script. If the user writes in English script but in a different language (like Hindi in English letters), respond in the same format. Match the user's linguistic style completely.\n\n"
            f"CODE EXECUTION OUTPUT INSTRUCTION: Don't just mention that you ran code - show the user what the code produced by writing the output yourself in the answer as text/markdown. That is copy the output and write it fully again as a normal response.\n\n"
            f"THINKING GUIDE:\n"
            f"1. Identify and clearly define the core agricultural problem or question\n"
            f"2. Reflect on whether the problem can be fully understood or if any assumptions need to be made. If assumptions are necessary, state them transparently. If there are any ambiguities or incomplete information, ask the user for clarification and stop the response\n"
            f"3. Once ambiguities are clear, break the problem down into smaller, logical parts or sub-questions, and solve them systematically\n"
            f"4. Search the internet and write code when required for data analysis or agricultural calculations\n"
            f"5. Before delivering your final answer, reflect on the steps taken. Ask yourself if the approach was thorough, if all aspects were addressed, and if there are any gaps in reasoning\n"
            f"6. Provide the comprehensive agricultural solution with clear explanations of why it's the best approach based on your analysis\n\n"
            f"User Query: {state.user_query}\n\n"
            f"Conversation History:\n{hist}\n\n"
            f"Available Agricultural Data & Tools:\n{json.dumps(state.tool_calls, indent=2)}\n"
            f"{files_context}\n\n"
            f"As SmartKrishi Agent, provide expert agricultural guidance following the thinking guide above."
        )
        
        # Configure tools - always include code execution so the model can run code when useful
        # (previously this was only enabled when CSV/XLSX files were present)
        tools = [
            SEARCH,
            types.Tool(url_context=types.UrlContext()),
            types.Tool(code_execution=types.ToolCodeExecution())
        ]

        # Prepare content including files and prompt
        contents = []
        if gemini_files:
            # Add uploaded files to the conversation
            contents.extend(gemini_files)
        contents.append(prompt_agent)

        if logs:
            yield json.dumps({
                "type": "log",
                "stage": "agent_prompt_ready",
                "message": f"🤔 Prompt prepared, sending to AI... Files: {len(gemini_files)}, Tools: {len(tools)}"
            }) + "\n"

        # Use client manager for file-aware generation (streaming version)
        from .client_manager import client_manager
        
        # capture internal tool events
        events_buffer = []
        def on_tool_request(request):  # type: ignore
            if getattr(request, 'google_search', None):
                events_buffer.append(
                    json.dumps({"type":"google_search_call","query":request.google_search.query}) + "\n"
                )
        def on_tool_response(response):  # type: ignore
            if getattr(response, 'google_search', None):
                events_buffer.append(
                    json.dumps({"type":"google_search_response","results":response.google_search.results}) + "\n"
                )

        config = types.GenerateContentConfig(
            thinking_config=types.ThinkingConfig(include_thoughts=True),
            tools=tools
        )
        
        # Collect the response and grounding data
        full_response = ""
        grounding_metadata = None
        
        # Use client manager's streaming method
        for chunk in client_manager.generate_content_stream(
            chat_id=chat_id,
            model=AGENT_MODEL,
            contents=contents,
            config=config,
        ):
            # emit any tool events
            while events_buffer:
                yield events_buffer.pop(0)
            candidates = getattr(chunk, 'candidates', None) or []
            if not candidates:
                continue
            candidate = candidates[0]
            
            # Extract grounding metadata if present
            if hasattr(candidate, 'grounding_metadata') and candidate.grounding_metadata:
                grounding_metadata = candidate.grounding_metadata
                
                # Extract and emit web search queries
                if hasattr(grounding_metadata, 'web_search_queries') and grounding_metadata.web_search_queries:
                    yield json.dumps({
                        "type": "grounding_web_search_queries",
                        "queries": list(grounding_metadata.web_search_queries)
                    }) + "\n"
                
                # Extract and emit grounding chunks (sources)
                if hasattr(grounding_metadata, 'grounding_chunks') and grounding_metadata.grounding_chunks:
                    chunks = []
                    for chunk_data in grounding_metadata.grounding_chunks:
                        if hasattr(chunk_data, 'web') and chunk_data.web:
                            chunks.append({
                                "uri": chunk_data.web.uri,
                                "title": chunk_data.web.title if hasattr(chunk_data.web, 'title') else "Unknown"
                            })
                    if chunks:
                        yield json.dumps({
                            "type": "grounding_chunks",
                            "sources": chunks
                        }) + "\n"
                
                # Extract and emit grounding supports (citations)
                if hasattr(grounding_metadata, 'grounding_supports') and grounding_metadata.grounding_supports:
                    supports = []
                    for support in grounding_metadata.grounding_supports:
                        if hasattr(support, 'segment') and hasattr(support, 'grounding_chunk_indices'):
                            supports.append({
                                "segment": {
                                    "start_index": support.segment.start_index if hasattr(support.segment, 'start_index') else 0,
                                    "end_index": support.segment.end_index if hasattr(support.segment, 'end_index') else 0,
                                    "text": support.segment.text if hasattr(support.segment, 'text') else ""
                                },
                                "grounding_chunk_indices": list(support.grounding_chunk_indices)
                            })
                    if supports:
                        yield json.dumps({
                            "type": "grounding_supports",
                            "supports": supports
                        }) + "\n"
            
            parts = getattr(candidate.content, 'parts', None) or []
            for part in parts:
                # Handle text parts
                text = getattr(part, 'text', None)
                if text:
                    if getattr(part, 'thought', False):
                        yield json.dumps({"type":"thinking","content":text}) + "\n"
                    else:
                        full_response += text
                        yield json.dumps({"type":"response_chunk","content":text}) + "\n"
                
                # Handle executable code parts
                if hasattr(part, 'executable_code') and part.executable_code:
                    executable_code = part.executable_code
                    code = getattr(executable_code, 'code', '')
                    language = getattr(executable_code, 'language', 'python')
                    
                    yield json.dumps({
                        "type": "code_execution",
                        "stage": "code",
                        "code": code,
                        "language": language
                    }) + "\n"
                
                # Handle code execution result parts
                if hasattr(part, 'code_execution_result') and part.code_execution_result:
                    result = part.code_execution_result
                    outcome = getattr(result, 'outcome', 'unknown')
                    output = getattr(result, 'output', '')
                    
                    yield json.dumps({
                        "type": "code_execution",
                        "stage": "result", 
                        "outcome": outcome,
                        "result": output
                    }) + "\n"

        # flush remaining tool events
        while events_buffer:
            yield events_buffer.pop(0)
            
        # Serialize grounding metadata properly
        serialized_grounding = None
        if grounding_metadata:
            serialized_grounding = {}
            
            if hasattr(grounding_metadata, 'web_search_queries') and grounding_metadata.web_search_queries:
                serialized_grounding["web_search_queries"] = list(grounding_metadata.web_search_queries)
            
            if hasattr(grounding_metadata, 'grounding_chunks') and grounding_metadata.grounding_chunks:
                chunks = []
                for chunk_data in grounding_metadata.grounding_chunks:
                    if hasattr(chunk_data, 'web') and chunk_data.web:
                        chunks.append({
                            "uri": chunk_data.web.uri,
                            "title": chunk_data.web.title if hasattr(chunk_data.web, 'title') else "Unknown"
                        })
                serialized_grounding["grounding_chunks"] = chunks
            
            if hasattr(grounding_metadata, 'grounding_supports') and grounding_metadata.grounding_supports:
                supports = []
                for support in grounding_metadata.grounding_supports:
                    if hasattr(support, 'segment') and support.segment and hasattr(support, 'grounding_chunk_indices') and support.grounding_chunk_indices:
                        supports.append({
                            "segment": {
                                "start_index": support.segment.start_index if hasattr(support.segment, 'start_index') else 0,
                                "end_index": support.segment.end_index if hasattr(support.segment, 'end_index') else 0,
                                "text": support.segment.text if hasattr(support.segment, 'text') else ""
                            },
                            "grounding_chunk_indices": list(support.grounding_chunk_indices)
                        })
                serialized_grounding["grounding_supports"] = supports
            
        # Emit final response
        yield json.dumps({
            "type": "response",
            "response": full_response,
            "grounding_metadata": serialized_grounding
        }) + "\n"
            
        # Save the complete response to chat history
        if full_response.strip():
            add_message(chat_id, user_id, "assistant", full_response)
            
        if logs:
            yield json.dumps({
                "type": "log",
                "stage": "complete",
                "message": "✅ Response generated successfully!"
            }) + "\n"
            
        yield json.dumps({"type":"end"}) + "\n"

    return StreamingResponse(event_generator(), media_type="application/json")

if __name__ == "__main__":
    os.makedirs("data",    exist_ok=True)
    os.makedirs("uploads", exist_ok=True)
    import uvicorn  # type: ignore
    uvicorn.run("app.main:api", host="0.0.0.0", port=8080, reload=True)
