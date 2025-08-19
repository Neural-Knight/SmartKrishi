from fastapi import APIRouter, Depends, HTTPException, UploadFile, File, Form, Query
from fastapi.security import HTTPBearer
from fastapi.responses import StreamingResponse
from sqlalchemy.orm import Session
from typing import List, Optional
from uuid import UUID
import logging
import json
import asyncio

from ..deps import get_current_user, get_db
from ..ai.chat_service import ChatService as AIService
from ..services.chat_service import ChatService
from ..services.integrated_chat_service import IntegratedChatService
from ..services.reasoning_service import ReasoningService
from ..services.file_service import FileService
from ..schemas.user import User
from ..schemas.chat import (
    SendMessageRequest, SendMessageResponse, CreateChatRequest,
    UpdateChatRequest, Chat, ChatSummary, ChatMessage, ChatMessageCreate
)
from ..schemas.reasoning import StreamingStatus, ReasoningStep
from ..schemas.file import FileUploadResponse, ChatFilesResponse, UploadedFile
from pydantic import BaseModel

logger = logging.getLogger(__name__)
router = APIRouter()
security = HTTPBearer()

# Instantiate services (reuse existing ones)
ai_service = AIService()
integrated_chat_service = IntegratedChatService()

# Common SSE headers
SSE_HEADERS = {
    "Cache-Control": "no-cache",
    "Connection": "keep-alive",
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Headers": "*",
}

# Legacy models for backward compatibility
class ChatMessageLegacy(BaseModel):
    message: str
    chat_history: Optional[List[dict]] = []

class ChatResponseLegacy(BaseModel):
    response: str
    message_id: Optional[str] = None

# ----- Existing endpoints (unchanged behavior) -----

@router.post("/ask", response_model=ChatResponseLegacy)
async def ask_question(
    chat_data: ChatMessageLegacy,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Process text-based farming question (legacy wrapper)."""
    try:
        if not chat_data.message.strip():
            raise HTTPException(status_code=400, detail="Message cannot be empty")

        response = await ai_service.process_text_message(
            message=chat_data.message,
            user_id=str(current_user.id),
            chat_history=chat_data.chat_history
        )

        return ChatResponseLegacy(response=response)

    except Exception as e:
        logger.exception(f"Chat error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to process your question")


@router.post("/analyze-image", response_model=ChatResponseLegacy)
async def analyze_crop_image(
    file: UploadFile = File(...),
    message: str = Form("Analyze this crop image"),
    current_user: User = Depends(get_current_user)
):
    """Analyze uploaded crop/farm image (legacy wrapper)."""
    try:
        if file.content_type not in ["image/jpeg", "image/png", "image/jpg", "image/webp"]:
            raise HTTPException(status_code=400, detail="Invalid file type. Please upload an image.")

        file_data = await file.read()
        if len(file_data) > 10 * 1024 * 1024:  # 10MB
            raise HTTPException(status_code=400, detail="File too large. Maximum size is 10MB.")

        response = await ai_service.process_image_message(
            message=message,
            image_data=file_data,
            image_mime_type=file.content_type,
            user_id=str(current_user.id)
        )

        return ChatResponseLegacy(response=response)

    except Exception as e:
        logger.exception(f"Image analysis error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to analyze image")


@router.get("/suggestions")
async def get_chat_suggestions(current_user: User = Depends(get_current_user)):
    """Get suggested questions for farmers"""
    suggestions = [
        {
            "id": "weather-advice",
            "text": "Weather Forecast Impact",
            "prompt": "How will the current weather conditions affect my farming activities?"
        },
        {
            "id": "crop-diseases",
            "text": "Disease Identification",
            "prompt": "Help me identify diseases affecting my crops and suggest treatments."
        },
        {
            "id": "soil-health",
            "text": "Soil Management",
            "prompt": "How can I improve my soil health for better crop yields?"
        },
        {
            "id": "market-prices",
            "text": "Market Insights",
            "prompt": "What are the current market trends for my crops?"
        },
        {
            "id": "pest-control",
            "text": "Pest Management",
            "prompt": "Suggest organic pest control methods for my crops."
        },
        {
            "id": "irrigation",
            "text": "Water Management",
            "prompt": "What's the best irrigation schedule for my current crops?"
        }
    ]
    return {"suggestions": suggestions}


# ----- Persistent chat endpoints (existing) -----

@router.post("/send", response_model=SendMessageResponse)
async def send_message(
    request: SendMessageRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Send a message and get AI response (non-streaming, legacy behavior)."""
    try:
        if not request.message.strip():
            raise HTTPException(status_code=400, detail="Message cannot be empty")

        # Get or create chat
        if request.chat_id:
            chat = ChatService.get_chat_by_id(db, request.chat_id, current_user.id)
            if not chat:
                raise HTTPException(status_code=404, detail="Chat not found")
        else:
            title = ChatService.generate_chat_title(request.message)
            chat = await ChatService.create_chat(db, current_user.id, title)

        # Get chat history for AI context
        messages = ChatService.get_chat_messages(db, chat.id, current_user.id)
        chat_history = [{"role": msg.role, "content": msg.content} for msg in messages]

        # Save user message
        user_message = ChatService.add_message(
            db, chat.id, current_user.id,
            ChatMessageCreate(role="user", content=request.message)
        )

        # Get AI response (synchronous wrapper that returns full response)
        ai_response = await ai_service.process_text_message(
            message=request.message,
            user_id=str(current_user.id),
            chat_history=chat_history
        )

        # Save AI response
        ai_message = ChatService.add_message(
            db, chat.id, current_user.id,
            ChatMessageCreate(role="assistant", content=ai_response)
        )

        return SendMessageResponse(
            response=ai_response,
            chat_id=chat.id,
            message_id=ai_message.id
        )

    except Exception as e:
        logger.exception(f"Send message error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to process your message")


@router.post("/analyze-image-persistent", response_model=SendMessageResponse)
async def analyze_crop_image_persistent(
    file: UploadFile = File(...),
    message: str = Form("Analyze this crop image"),
    chat_id: Optional[str] = Form(None),
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Analyze uploaded crop image with persistence (non-streaming)."""
    try:
        if file.content_type not in ["image/jpeg", "image/png", "image/jpg", "image/webp"]:
            raise HTTPException(status_code=400, detail="Invalid file type. Please upload an image.")

        file_data = await file.read()
        if len(file_data) > 10 * 1024 * 1024:
            raise HTTPException(status_code=400, detail="File too large. Maximum size is 10MB.")

        # Get or create chat
        chat_uuid = None
        if chat_id:
            try:
                chat_uuid = UUID(chat_id)
                chat = ChatService.get_chat_by_id(db, chat_uuid, current_user.id)
                if not chat:
                    raise HTTPException(status_code=404, detail="Chat not found")
            except ValueError:
                raise HTTPException(status_code=400, detail="Invalid chat ID format")
        else:
            title = f"Image Analysis: {file.filename}"
            chat = await ChatService.create_chat(db, current_user.id, title)
            chat_uuid = chat.id

        # Save user message with image info
        user_message_content = f"📷 {message}\n[Uploaded image: {file.filename}]"
        user_message = ChatService.add_message(
            db, chat_uuid, current_user.id,
            ChatMessageCreate(
                role="user",
                content=user_message_content,
                message_type="image",
                file_url=file.filename
            )
        )

        # Get AI response
        ai_response = await ai_service.process_image_message(
            message=message,
            image_data=file_data,
            image_mime_type=file.content_type,
            user_id=str(current_user.id)
        )

        # Save AI response
        ai_message = ChatService.add_message(
            db, chat_uuid, current_user.id,
            ChatMessageCreate(role="assistant", content=ai_response)
        )

        return SendMessageResponse(
            response=ai_response,
            chat_id=chat_uuid,
            message_id=ai_message.id
        )

    except Exception as e:
        logger.exception(f"Image analysis error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to analyze image")


@router.get("/chats", response_model=List[ChatSummary])
async def get_user_chats(
    skip: int = Query(0, ge=0),
    limit: int = Query(50, ge=1, le=100),
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get all chats for the current user"""
    return ChatService.get_user_chats(db, current_user.id, skip, limit)


@router.get("/chats/{chat_id}", response_model=Chat)
async def get_chat(
    chat_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get a specific chat with all messages"""
    chat = ChatService.get_chat_by_id(db, chat_id, current_user.id)
    if not chat:
        raise HTTPException(status_code=404, detail="Chat not found")
    return chat


@router.post("/chats", response_model=Chat)
async def create_chat(
    request: CreateChatRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Create a new chat"""
    chat = await ChatService.create_chat(db, current_user.id, request.title)
    return chat


@router.put("/chats/{chat_id}", response_model=Chat)
async def update_chat(
    chat_id: UUID,
    request: UpdateChatRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Update chat title"""
    from ..schemas.chat import ChatUpdate
    chat = ChatService.update_chat(
        db, chat_id, current_user.id,
        ChatUpdate(title=request.title)
    )
    if not chat:
        raise HTTPException(status_code=404, detail="Chat not found")
    return chat


@router.delete("/chats/{chat_id}")
async def delete_chat(
    chat_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Delete a chat"""
    success = ChatService.delete_chat(db, chat_id, current_user.id)
    if not success:
        raise HTTPException(status_code=404, detail="Chat not found")
    return {"message": "Chat deleted successfully"}


# ----- Streaming integration endpoints (single, consistent versions) -----

class StreamingMessageRequest(BaseModel):
    message: str
    chat_id: Optional[UUID] = None
    model: Optional[str] = None
    tools: Optional[List[str]] = None
    include_logs: Optional[bool] = True


@router.post("/send-stream")
async def send_message_stream(
    request: StreamingMessageRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """
    Send a message with streaming response and reasoning persistence.

    The backend will:
    - create / fetch chat
    - persist user message
    - call integrated_chat_service.send_message_stream(...) which yields event dicts
    - stream each event to the frontend as SSE (data: <json>\n\n)
    - yield a final {"type": "end"} event when done
    """
    try:
        if not request.message or not request.message.strip():
            raise HTTPException(status_code=400, detail="Message cannot be empty")

        # Get or create chat
        if request.chat_id:
            chat = ChatService.get_chat_by_id(db, request.chat_id, current_user.id)
            if not chat:
                raise HTTPException(status_code=404, detail="Chat not found")
            chat_id = request.chat_id
        else:
            title = ChatService.generate_chat_title(request.message)
            chat = await ChatService.create_chat(db, current_user.id, title)
            chat_id = chat.id

        # Persist user message
        user_message = ChatService.add_message(
            db, chat_id, current_user.id,
            ChatMessageCreate(role="user", content=request.message)
        )

        async def stream_generator():
            try:
                # integrated_chat_service.send_message_stream should be an async generator
                # yielding dict events like {"type": "plan", ...}, {"type": "tool_call", ...}, {"type": "response_chunk", ...}
                async for event in integrated_chat_service.send_message_stream(
                    db=db,
                    user_id=current_user.id,
                    chat_id=chat_id,
                    message=request.message,
                    model=request.model,
                    tools=request.tools,
                    include_logs=request.include_logs
                ):
                    # Normalize event to include chat_id/message_id if possible
                    if isinstance(event, dict):
                        if "chat_id" not in event:
                            event["chat_id"] = str(chat_id)
                        # Optionally attach user_message id or assistant message id if integrated_chat_service sets them
                    else:
                        # ensure event is JSON-serializable; convert to dict
                        event = {"type": "log", "message": str(event), "chat_id": str(chat_id)}

                    # Yield as SSE 'data' block
                    yield f"data: {json.dumps(event)}\n\n"

                # Finalize
                yield f"data: {json.dumps({'type': 'end', 'chat_id': str(chat_id)})}\n\n"

            except Exception as e:
                logger.exception(f"Streaming error for user {current_user.id}: {e}")
                error_event = {
                    "type": "error",
                    "error": str(e),
                    "chat_id": str(chat_id)
                }
                yield f"data: {json.dumps(error_event)}\n\n"
                # also send final end so frontend can finalize UI
                yield f"data: {json.dumps({'type': 'end', 'chat_id': str(chat_id)})}\n\n"

        return StreamingResponse(
            stream_generator(),
            media_type="text/event-stream",
            headers=SSE_HEADERS
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception(f"Send stream message error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to process your message")


@router.post("/upload-and-analyze-stream")
async def upload_file_and_analyze_stream(
    file: UploadFile = File(...),
    message: str = Form("Analyze this file"),
    chat_id: Optional[str] = Form(None),
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """
    Upload a file and analyze it using the agent pipeline, streaming reasoning + response via SSE.
    """
    try:
        allowed_types = [
            "image/jpeg", "image/png", "image/jpg", "image/webp", "image/heic", "image/heif",
            "application/pdf",
            "application/vnd.openxmlformats-officedocument.wordprocessingml.document", # DOCX
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",      # XLSX
            "application/vnd.ms-excel",  # XLS
            "text/csv",                  # CSV
            "text/plain"                 # TXT
        ]

        if file.content_type not in allowed_types:
            raise HTTPException(status_code=400, detail="Invalid file type. Supported: images (PNG, JPG, JPEG, WebP, HEIC, HEIF), PDF, DOCX, XLSX, CSV")

        file_data = await file.read()
        if len(file_data) > 20 * 1024 * 1024:  # 20MB
            raise HTTPException(status_code=400, detail="File too large. Maximum size is 20MB.")

        # Get or create chat
        if chat_id:
            chat_uuid = UUID(chat_id)
            chat = ChatService.get_chat_by_id(db, chat_uuid, current_user.id)
            if not chat:
                raise HTTPException(status_code=404, detail="Chat not found")
        else:
            title = f"File Analysis: {file.filename}"
            chat = await ChatService.create_chat(db, current_user.id, title)
            chat_uuid = chat.id

        async def stream_generator():
            try:
                async for event in integrated_chat_service.upload_file_and_analyze(
                    db=db,
                    user_id=current_user.id,
                    chat_id=chat_uuid,
                    file_data=file_data,
                    filename=file.filename or "uploaded_file",
                    message=message
                ):
                    if isinstance(event, dict):
                        if "chat_id" not in event:
                            event["chat_id"] = str(chat_uuid)
                    else:
                        event = {"type": "log", "message": str(event), "chat_id": str(chat_uuid)}

                    yield f"data: {json.dumps(event)}\n\n"

                yield f"data: {json.dumps({'type': 'end', 'chat_id': str(chat_uuid)})}\n\n"

            except Exception as e:
                logger.exception(f"File analysis streaming error for user {current_user.id}: {e}")
                error_event = {
                    "type": "error",
                    "error": str(e),
                    "chat_id": str(chat_uuid)
                }
                yield f"data: {json.dumps(error_event)}\n\n"
                yield f"data: {json.dumps({'type': 'end', 'chat_id': str(chat_uuid)})}\n\n"

        return StreamingResponse(
            stream_generator(),
            media_type="text/event-stream",
            headers=SSE_HEADERS
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception(f"Upload and analyze error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to process file")


# ----- Reasoning retrieval endpoints (single definitions) -----

@router.get("/chats/{chat_id}/reasoning")
async def get_chat_reasoning(
    chat_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get reasoning steps for a chat"""
    try:
        chat = ChatService.get_chat_by_id(db, chat_id, current_user.id)
        if not chat:
            raise HTTPException(status_code=404, detail="Chat not found")

        reasoning_steps = ReasoningService.get_reasoning_steps_for_chat(db, chat_id, current_user.id)
        return {"chat_id": str(chat_id), "reasoning_steps": reasoning_steps}

    except Exception as e:
        logger.exception(f"Get chat reasoning error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get chat reasoning")


@router.get("/messages/{message_id}/reasoning")
async def get_message_reasoning(
    message_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get reasoning steps for a specific message"""
    try:
        message = ChatService.get_message_by_id(db, message_id, current_user.id)
        if not message:
            raise HTTPException(status_code=404, detail="Message not found")

        reasoning_steps = ReasoningService.get_reasoning_steps_for_message(db, message_id, current_user.id)
        return {"message_id": str(message_id), "reasoning_steps": reasoning_steps}

    except Exception as e:
        logger.exception(f"Get message reasoning error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get message reasoning")


# ----- Agent tools & config endpoints -----

@router.get("/agent-tools")
async def get_agent_tools(current_user: User = Depends(get_current_user)):
    """Get available tools from the agent API"""
    try:
        tools = await integrated_chat_service.get_available_tools()
        return {"tools": tools}
    except Exception as e:
        logger.exception(f"Get agent tools error for user {current_user.id}: {e}")
        return {"tools": []}


@router.get("/agent-config")
async def get_user_agent_config(
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get user's agent API configuration"""
    try:
        config = ReasoningService.get_or_create_user_agent_config(db, current_user.id)
        return config
    except Exception as e:
        logger.exception(f"Get agent config error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to get agent configuration")


class AgentConfigUpdateRequest(BaseModel):
    preferred_model: Optional[str] = None
    default_tools: Optional[List[str]] = None
    include_logs: Optional[bool] = None


@router.put("/agent-config")
async def update_user_agent_config(
    request: AgentConfigUpdateRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Update user's agent API configuration"""
    try:
        from ..schemas.reasoning import AgentApiConfigUpdate, AgentApiConfigCreate

        update_data = AgentApiConfigUpdate(**request.dict(exclude_unset=True))
        config = ReasoningService.update_user_agent_config(db, current_user.id, update_data)

        if not config:
            create_data = AgentApiConfigCreate(user_id=current_user.id, **request.dict(exclude_unset=True))
            config = ReasoningService.create_user_agent_config(db, create_data)

        return config

    except Exception as e:
        logger.exception(f"Update agent config error for user {current_user.id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to update agent configuration")


# ----- File Management Endpoints -----

@router.post("/upload-file", response_model=FileUploadResponse)
async def upload_file(
    chat_id: UUID = Form(...),
    message_id: Optional[UUID] = Form(None),
    file: UploadFile = File(...),
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Upload a file to a chat"""
    try:
        file_service = FileService(db)
        
        # Validate file type
        if not file_service.is_file_type_allowed(file.filename):
            raise HTTPException(
                status_code=400, 
                detail=f"File type not allowed. Allowed types: {file_service.get_allowed_file_types()}"
            )
        
        # Read file data
        file_data = await file.read()
        
        # Check file size (10MB limit)
        if len(file_data) > 10 * 1024 * 1024:
            raise HTTPException(status_code=400, detail="File too large. Maximum size is 10MB.")
        
        # Upload file
        result = await file_service.upload_file(
            user_id=current_user.id,
            chat_id=chat_id,
            file_data=file_data,
            filename=file.filename,
            mime_type=file.content_type,
            message_id=message_id
        )
        
        return result
        
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"File upload error: {e}")
        raise HTTPException(status_code=500, detail="Failed to upload file")


@router.get("/chats/{chat_id}/files", response_model=ChatFilesResponse)
async def get_chat_files(
    chat_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get all files for a specific chat"""
    try:
        file_service = FileService(db)
        return file_service.get_chat_files(current_user.id, chat_id)
    except Exception as e:
        logger.error(f"Get chat files error: {e}")
        raise HTTPException(status_code=500, detail="Failed to get chat files")


@router.get("/files/{file_id}", response_model=UploadedFile)
async def get_file(
    file_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Get a specific file by ID"""
    try:
        file_service = FileService(db)
        file = file_service.get_file_by_id(current_user.id, file_id)
        if not file:
            raise HTTPException(status_code=404, detail="File not found")
        return file
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Get file error: {e}")
        raise HTTPException(status_code=500, detail="Failed to get file")


@router.delete("/files/{file_id}")
async def delete_file(
    file_id: UUID,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """Delete a file"""
    try:
        file_service = FileService(db)
        if not file_service.delete_file(current_user.id, file_id):
            raise HTTPException(status_code=404, detail="File not found")
        return {"message": "File deleted successfully", "file_id": str(file_id)}
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Delete file error: {e}")
        raise HTTPException(status_code=500, detail="Failed to delete file")
