from sqlalchemy import Column, Integer, String, Text, DateTime, ForeignKey, Boolean, BigInteger
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship
from sqlalchemy.sql import func
import uuid

from app.db.database import Base

class UploadedFile(Base):
    __tablename__ = "uploaded_files"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    chat_id = Column(UUID(as_uuid=True), ForeignKey("chats.id"), nullable=False)
    message_id = Column(UUID(as_uuid=True), ForeignKey("chat_messages.id"), nullable=True)  # Link to the message that contains this file
    
    # File metadata
    original_filename = Column(String(255), nullable=False)
    file_type = Column(String(50), nullable=False)  # png, jpg, jpeg, webp, heic, heif, pdf, docx, xlsx, csv
    file_size = Column(BigInteger, nullable=False)  # File size in bytes
    mime_type = Column(String(100), nullable=True)
    
    # Agent API integration
    agent_file_id = Column(String(255), nullable=True)  # File ID from Agent API
    processing_status = Column(String(50), nullable=False, default='uploaded')  # uploaded, processing, completed, failed
    
    # File content summary (extracted from Agent API response)
    summary = Column(Text, nullable=True)
    file_metadata = Column(Text, nullable=True)  # JSON string for additional metadata
    
    # Timestamps
    created_at = Column(DateTime(timezone=True), server_default=func.now())
    updated_at = Column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())
    is_deleted = Column(Boolean, nullable=False, default=False, server_default='false')
    
    # Relationships
    user = relationship("User")
    chat = relationship("Chat")
    message = relationship("ChatMessage", back_populates="files")
