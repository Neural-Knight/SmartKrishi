from typing import Optional, List
from sqlalchemy.orm import Session
from uuid import UUID
import json
import logging

from app.models.file import UploadedFile
from app.models.chat import Chat
from app.schemas.file import UploadedFileCreate, UploadedFileUpdate, FileUploadResponse, ChatFilesResponse
from app.services.agent_api_service import AgentAPIService

logger = logging.getLogger(__name__)

class FileService:
    def __init__(self, db: Session):
        self.db = db
        self.agent_api_service = AgentAPIService()

    def get_allowed_file_types(self) -> dict:
        """Get allowed file types and their extensions"""
        return {
            "images": [".png", ".jpg", ".jpeg", ".webp", ".heic", ".heif"],
            "pdf": [".pdf"],
            "documents": [".docx"],
            "spreadsheets": [".xlsx"],
            "data": [".csv"]
        }

    def is_file_type_allowed(self, filename: str) -> bool:
        """Check if file type is allowed"""
        allowed_extensions = []
        for category in self.get_allowed_file_types().values():
            allowed_extensions.extend(category)
        
        return any(filename.lower().endswith(ext) for ext in allowed_extensions)

    def get_file_type_from_filename(self, filename: str) -> Optional[str]:
        """Extract file type from filename - returns specific extension"""
        filename_lower = filename.lower()
        
        if filename_lower.endswith(".png"):
            return "png"
        elif filename_lower.endswith(".jpg") or filename_lower.endswith(".jpeg"):
            return "jpg"
        elif filename_lower.endswith(".webp"):
            return "webp"
        elif filename_lower.endswith(".heic"):
            return "heic"
        elif filename_lower.endswith(".heif"):
            return "heif"
        elif filename_lower.endswith(".pdf"):
            return "pdf"
        elif filename_lower.endswith(".docx"):
            return "docx"
        elif filename_lower.endswith(".xlsx"):
            return "xlsx"
        elif filename_lower.endswith(".csv"):
            return "csv"
        return None

    async def save_file_to_database(
        self,
        user_id: int,
        chat_id: UUID,
        file_data: bytes,
        filename: str,
        mime_type: str,
        agent_file_id: str,
        message_id: Optional[UUID] = None
    ) -> FileUploadResponse:
        """Save file metadata to database without uploading to agent API"""
        
        # Validate file type
        if not self.is_file_type_allowed(filename):
            raise ValueError(f"File type not allowed. Allowed types: {self.get_allowed_file_types()}")
        
        file_type = self.get_file_type_from_filename(filename)
        if not file_type:
            raise ValueError("Could not determine file type")

        # Create file record in database
        file_create = UploadedFileCreate(
            user_id=user_id,
            chat_id=chat_id,
            message_id=message_id,
            original_filename=filename,
            file_type=file_type,
            file_size=len(file_data),
            mime_type=mime_type,
            agent_file_id=agent_file_id,
            processing_status="uploaded"
        )
        
        db_file = UploadedFile(**file_create.model_dump())
        self.db.add(db_file)
        self.db.commit()
        self.db.refresh(db_file)

        return FileUploadResponse(
            file_id=db_file.id,
            original_filename=db_file.original_filename,
            file_type=db_file.file_type,
            file_size=db_file.file_size,
            processing_status=db_file.processing_status,
            agent_file_id=db_file.agent_file_id,
            message="File uploaded successfully"
        )

    async def upload_file(
        self,
        user_id: int,
        chat_id: UUID,
        file_data: bytes,
        filename: str,
        mime_type: str,
        message_id: Optional[UUID] = None
    ) -> FileUploadResponse:
        """Upload file to agent API and save metadata to database"""
        
        # Validate file type
        if not self.is_file_type_allowed(filename):
            raise ValueError(f"File type not allowed. Allowed types: {self.get_allowed_file_types()}")
        
        file_type = self.get_file_type_from_filename(filename)
        if not file_type:
            raise ValueError("Could not determine file type")

        # Create file record in database
        file_create = UploadedFileCreate(
            user_id=user_id,
            chat_id=chat_id,
            message_id=message_id,
            original_filename=filename,
            file_type=file_type,
            file_size=len(file_data),
            mime_type=mime_type
        )
        
        db_file = UploadedFile(**file_create.model_dump())
        self.db.add(db_file)
        self.db.commit()
        self.db.refresh(db_file)

        try:
            # Get chat for agent_chat_id
            chat = self.db.query(Chat).filter(Chat.id == chat_id).first()
            if not chat or not chat.agent_chat_id:
                raise ValueError("Chat not found or no agent_chat_id")

            # Upload to agent API
            agent_response = await self.agent_api_service.upload_file(
                user_id=str(user_id),
                chat_id=chat.agent_chat_id,
                file_data=file_data,
                filename=filename,
                file_type=file_type
            )

            # Update file record with agent response
            update_data = UploadedFileUpdate(
                agent_file_id=agent_response.get("file_id"),
                processing_status=agent_response.get("processing_status", "processing"),
                summary=agent_response.get("message")
            )

            for key, value in update_data.model_dump(exclude_unset=True).items():
                setattr(db_file, key, value)
            
            self.db.commit()
            self.db.refresh(db_file)

            return FileUploadResponse(
                file_id=db_file.id,
                original_filename=db_file.original_filename,
                file_type=db_file.file_type,
                file_size=db_file.file_size,
                processing_status=db_file.processing_status,
                agent_file_id=db_file.agent_file_id,
                message=agent_response.get("message", "File uploaded successfully")
            )

        except Exception as e:
            # Update file status to failed
            db_file.processing_status = "failed"
            self.db.commit()
            logger.error(f"Failed to upload file to agent API: {e}")
            raise

    def get_chat_files(self, user_id: int, chat_id: UUID) -> ChatFilesResponse:
        """Get all files for a specific chat"""
        files = self.db.query(UploadedFile).filter(
            UploadedFile.user_id == user_id,
            UploadedFile.chat_id == chat_id,
            UploadedFile.is_deleted == False
        ).all()

        return ChatFilesResponse(
            user_id=user_id,
            chat_id=chat_id,
            files=files,
            total_count=len(files)
        )

    def get_message_files(self, message_id: UUID) -> List[UploadedFile]:
        """Get all files for a specific message"""
        return self.db.query(UploadedFile).filter(
            UploadedFile.message_id == message_id,
            UploadedFile.is_deleted == False
        ).all()

    def delete_file(self, user_id: int, file_id: UUID) -> bool:
        """Soft delete a file"""
        file = self.db.query(UploadedFile).filter(
            UploadedFile.id == file_id,
            UploadedFile.user_id == user_id
        ).first()
        
        if not file:
            return False
        
        file.is_deleted = True
        self.db.commit()
        return True

    def get_file_by_id(self, user_id: int, file_id: UUID) -> Optional[UploadedFile]:
        """Get a specific file by ID"""
        return self.db.query(UploadedFile).filter(
            UploadedFile.id == file_id,
            UploadedFile.user_id == user_id,
            UploadedFile.is_deleted == False
        ).first()
