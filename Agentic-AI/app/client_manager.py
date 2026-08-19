"""
Gemini Client Manager - Ensures consistent client usage for file operations
and maintains proper isolation between users/chats.
"""

import os
import json
import logging
from typing import Dict, Optional, Any
from google import genai
from google.genai import types
import time
from threading import Lock

log = logging.getLogger("ClientManager")

class GeminiClientManager:
    """
    Manages Gemini clients per chat to ensure file access consistency.
    Files uploaded by a client can only be accessed by the same client.
    """
    
    def __init__(self):
        self.clients: Dict[str, genai.Client] = {}
        self.client_lock = Lock()
        self.api_key = os.getenv('GOOGLE_API_KEY') or os.getenv('GEMINI_API_KEY')
        
        if not self.api_key:
            log.warning("No GEMINI_API_KEY found - file operations will be limited")
        
        # File metadata for persistence
        self.file_registry_path = "data/gemini_files_registry.json"
        os.makedirs(os.path.dirname(self.file_registry_path), exist_ok=True)
        self.file_registry = self._load_file_registry()
    
    def _load_file_registry(self) -> Dict:
        """Load the file registry from disk"""
        try:
            if os.path.exists(self.file_registry_path):
                with open(self.file_registry_path, 'r') as f:
                    return json.load(f)
        except Exception as e:
            log.warning(f"Failed to load file registry: {e}")
        return {}
    
    def _save_file_registry(self):
        """Save the file registry to disk"""
        try:
            with open(self.file_registry_path, 'w') as f:
                json.dump(self.file_registry, f, indent=2)
        except Exception as e:
            log.error(f"Failed to save file registry: {e}")
    
    def get_client(self, chat_id: str) -> Optional[genai.Client]:
        """
        Get or create a Gemini client for a specific chat.
        This ensures file access consistency within a chat.
        """
        if not self.api_key:
            log.warning("No API key available for Gemini client")
            return None
        
        with self.client_lock:
            if chat_id not in self.clients:
                try:
                    self.clients[chat_id] = genai.Client(api_key=self.api_key)
                    log.info(f"Created new Gemini client for chat {chat_id}")
                except Exception as e:
                    log.error(f"Failed to create Gemini client for chat {chat_id}: {e}")
                    return None
            
            return self.clients[chat_id]
    
    def upload_file(self, chat_id: str, file_path: str, display_name: Optional[str] = None, mime_type: Optional[str] = None) -> Optional[Any]:
        """
        Upload a file using the chat's dedicated client and register it.
        """
        client = self.get_client(chat_id)
        if not client:
            return None
        
        try:
            # Prepare upload config
            config = {}
            if display_name:
                config["display_name"] = display_name
            if mime_type:
                config["mime_type"] = mime_type
            
            # Upload file to Gemini
            if config:
                uploaded_file = client.files.upload(file=file_path, config=config)  # type: ignore
            else:
                uploaded_file = client.files.upload(file=file_path)
            
            # Register the file for persistence
            file_info = {
                "name": uploaded_file.name,
                "display_name": uploaded_file.display_name,
                "file_path": file_path,
                "chat_id": chat_id,
                "upload_time": time.time(),
                "size_bytes": getattr(uploaded_file, 'size_bytes', 0),
                "mime_type": getattr(uploaded_file, 'mime_type', 'unknown')
            }
            
            self.file_registry[uploaded_file.name] = file_info
            self._save_file_registry()
            
            log.info(f"Successfully uploaded file {display_name} for chat {chat_id}")
            return uploaded_file
            
        except Exception as e:
            log.error(f"Failed to upload file {file_path} for chat {chat_id}: {e}")
            return None
    
    def get_file(self, chat_id: str, file_name: str) -> Optional[Any]:
        """
        Get a file reference using the chat's dedicated client.
        """
        client = self.get_client(chat_id)
        if not client:
            return None
        
        try:
            return client.files.get(name=file_name)
        except Exception as e:
            log.warning(f"Failed to get file {file_name} for chat {chat_id}: {e}")
            
            # Try to re-upload if file exists locally
            if file_name in self.file_registry:
                file_info = self.file_registry[file_name]
                local_path = file_info.get('file_path')
                if local_path and os.path.exists(local_path):
                    log.info(f"Attempting to re-upload file {file_name}")
                    return self.upload_file(chat_id, local_path, file_info.get('display_name'))
            
            return None
    
    def list_chat_files(self, chat_id: str) -> list:
        """
        List all files for a specific chat.
        """
        client = self.get_client(chat_id)
        if not client:
            return []
        
        try:
            # Get files from the registry that belong to this chat
            chat_files = []
            for file_name, file_info in self.file_registry.items():
                if file_info.get('chat_id') == chat_id:
                    try:
                        # Verify file still exists in Gemini
                        file_ref = client.files.get(name=file_name)
                        chat_files.append(file_ref)
                    except Exception:
                        # File may have expired, try to re-upload if local copy exists
                        local_path = file_info.get('file_path')
                        if local_path and os.path.exists(local_path):
                            log.info(f"Re-uploading expired file {file_name}")
                            new_file = self.upload_file(chat_id, local_path, file_info.get('display_name'))
                            if new_file:
                                chat_files.append(new_file)
            
            return chat_files
            
        except Exception as e:
            log.error(f"Failed to list files for chat {chat_id}: {e}")
            return []
    
    def generate_content(self, chat_id: str, contents: list, config: Optional[types.GenerateContentConfig] = None, model: str = "gemini-2.5-flash") -> Optional[Any]:
        """
        Generate content using the chat's dedicated client.
        This ensures access to files uploaded by the same client.
        """
        client = self.get_client(chat_id)
        if not client:
            return None
        
        try:
            return client.models.generate_content(
                model=model,
                contents=contents,
                config=config
            )
        except Exception as e:
            log.error(f"Failed to generate content for chat {chat_id}: {e}")
            return None
    
    def generate_content_stream(self, chat_id: str, contents: list, config: Optional[types.GenerateContentConfig] = None, model: str = "gemini-2.5-flash"):
        """
        Generate streaming content using the chat's dedicated client.
        This ensures access to files uploaded by the same client.
        """
        client = self.get_client(chat_id)
        if not client:
            return iter([])  # Return empty iterator if no client
        
        try:
            return client.models.generate_content_stream(
                model=model,
                contents=contents,
                config=config
            )
        except Exception as e:
            log.error(f"Failed to generate streaming content for chat {chat_id}: {e}")
            return iter([])  # Return empty iterator on error
    
    def cleanup_expired_files(self, max_age_hours: int = 48):
        """
        Clean up files that are older than max_age_hours.
        Gemini files expire after 48 hours by default.
        """
        current_time = time.time()
        expired_files = []
        
        for file_name, file_info in list(self.file_registry.items()):
            upload_time = file_info.get('upload_time', 0)
            age_hours = (current_time - upload_time) / 3600
            
            if age_hours > max_age_hours:
                expired_files.append(file_name)
                del self.file_registry[file_name]
        
        if expired_files:
            self._save_file_registry()
            log.info(f"Cleaned up {len(expired_files)} expired files")
        
        return expired_files

# Global instance
client_manager = GeminiClientManager()
