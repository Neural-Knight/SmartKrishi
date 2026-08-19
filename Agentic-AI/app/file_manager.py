import sqlite3
import os
import logging
from typing import List, Dict, Optional, Any
from datetime import datetime

log = logging.getLogger("FileManager")
DB_PATH = os.getenv("FILES_DB", "data/files.db")
os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)

conn = sqlite3.connect(DB_PATH, check_same_thread=False)

# Create file metadata table
conn.execute("""CREATE TABLE IF NOT EXISTS uploaded_files(
        file_id TEXT PRIMARY KEY,
        chat_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        original_filename TEXT NOT NULL,
        stored_path TEXT NOT NULL,
        file_type TEXT NOT NULL,
        file_size INTEGER,
        gemini_file_id TEXT,
        summary TEXT,
        content_preview TEXT,
        upload_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        processing_status TEXT DEFAULT 'pending'
        )""")

conn.commit()

def store_file_metadata(
    file_id: str,
    chat_id: str, 
    user_id: str,
    original_filename: str,
    stored_path: str,
    file_type: str,
    file_size: int,
    gemini_file_id: Optional[str] = None,
    summary: Optional[str] = None,
    content_preview: Optional[str] = None,
    processing_status: str = "pending"
) -> None:
    """Store file metadata in database"""
    conn.execute("""
        INSERT INTO uploaded_files 
        (file_id, chat_id, user_id, original_filename, stored_path, file_type, 
         file_size, gemini_file_id, summary, content_preview, processing_status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    """, (file_id, chat_id, user_id, original_filename, stored_path, file_type,
          file_size, gemini_file_id, summary, content_preview, processing_status))
    conn.commit()

def update_file_processing(
    file_id: str,
    gemini_file_ref: Optional[Any] = None,
    summary: Optional[str] = None,
    content_preview: Optional[str] = None,
    processing_status: str = "completed"
) -> None:
    """Update file after processing"""
    # Store the file reference name if it's a Gemini file object
    gemini_file_id = None
    if gemini_file_ref and hasattr(gemini_file_ref, 'name'):
        gemini_file_id = gemini_file_ref.name
    
    conn.execute("""
        UPDATE uploaded_files 
        SET gemini_file_id=?, summary=?, content_preview=?, processing_status=?
        WHERE file_id=?
    """, (gemini_file_id, summary, content_preview, processing_status, file_id))
    conn.commit()

def get_file_metadata(file_id: str, user_id: str) -> Optional[Dict]:
    """Get file metadata by ID"""
    row = conn.execute("""
        SELECT file_id, chat_id, user_id, original_filename, stored_path, file_type,
               file_size, gemini_file_id, summary, content_preview, upload_timestamp, processing_status
        FROM uploaded_files 
        WHERE file_id=? AND user_id=?
    """, (file_id, user_id)).fetchone()
    
    if row:
        return {
            "file_id": row[0], "chat_id": row[1], "user_id": row[2],
            "original_filename": row[3], "stored_path": row[4], "file_type": row[5],
            "file_size": row[6], "gemini_file_id": row[7], "summary": row[8],
            "content_preview": row[9], "upload_timestamp": row[10], "processing_status": row[11]
        }
    return None

def get_chat_files(chat_id: str, user_id: str) -> List[Dict]:
    """Get all files for a specific chat"""
    rows = conn.execute("""
        SELECT file_id, original_filename, file_type, file_size, summary, 
               content_preview, upload_timestamp, processing_status, gemini_file_id
        FROM uploaded_files 
        WHERE chat_id=? AND user_id=? 
        ORDER BY upload_timestamp DESC
    """, (chat_id, user_id)).fetchall()
    
    return [{
        "file_id": row[0], "original_filename": row[1], "file_type": row[2],
        "file_size": row[3], "summary": row[4], "content_preview": row[5],
        "upload_timestamp": row[6], "processing_status": row[7], "gemini_file_id": row[8]
    } for row in rows]

def get_user_files(user_id: str, limit: int = 50) -> List[Dict]:
    """Get all files for a user across all chats"""
    rows = conn.execute("""
        SELECT file_id, chat_id, original_filename, file_type, file_size, summary,
               upload_timestamp, processing_status
        FROM uploaded_files 
        WHERE user_id=? 
        ORDER BY upload_timestamp DESC 
        LIMIT ?
    """, (user_id, limit)).fetchall()
    
    return [{
        "file_id": row[0], "chat_id": row[1], "original_filename": row[2], 
        "file_type": row[3], "file_size": row[4], "summary": row[5],
        "upload_timestamp": row[6], "processing_status": row[7]
    } for row in rows]

def delete_file_metadata(file_id: str, user_id: str) -> bool:
    """Delete file metadata (and optionally the file itself)"""
    # Get file info first
    file_info = get_file_metadata(file_id, user_id)
    if not file_info:
        return False
    
    # Delete from database
    result = conn.execute("""
        DELETE FROM uploaded_files 
        WHERE file_id=? AND user_id=?
    """, (file_id, user_id))
    conn.commit()
    
    # Optionally delete physical file
    try:
        if os.path.exists(file_info["stored_path"]):
            os.remove(file_info["stored_path"])
            log.info(f"Deleted file: {file_info['stored_path']}")
    except Exception as e:
        log.warning(f"Could not delete physical file: {e}")
    
    return result.rowcount > 0

def search_files(user_id: str, query: str, chat_id: Optional[str] = None) -> List[Dict]:
    """Search files by filename or summary"""
    base_query = """
        SELECT file_id, chat_id, original_filename, file_type, summary, upload_timestamp
        FROM uploaded_files 
        WHERE user_id=? AND (original_filename LIKE ? OR summary LIKE ?)
    """
    params = [user_id, f"%{query}%", f"%{query}%"]
    
    if chat_id:
        base_query += " AND chat_id=?"
        params.append(chat_id)
    
    base_query += " ORDER BY upload_timestamp DESC"
    
    rows = conn.execute(base_query, params).fetchall()
    
    return [{
        "file_id": row[0], "chat_id": row[1], "original_filename": row[2],
        "file_type": row[3], "summary": row[4], "upload_timestamp": row[5]
    } for row in rows]
