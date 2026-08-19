import logging
from typing import Dict, Any
from ..history import search_messages, get_chat_messages, get_user_chats

log = logging.getLogger("ChatHistoryTool")

def chat_history_tool(query: str, user_id: str = "", chat_id: str = "", limit: int = 10) -> Dict[str, Any]:
    """
    Tool for Gemini to fetch chat history. Can search across all chats or within specific chat.
    
    Args:
        query: Search query for messages (optional, if empty returns recent messages)
        user_id: User ID (required)
        chat_id: Specific chat ID (optional, if empty searches all user's chats)
        limit: Maximum number of messages to return
    """
    try:
        if not user_id:
            return {"error": "user_id is required"}
        
        if query.strip():
            # Search for specific messages
            messages = search_messages(user_id, query, chat_id, limit)
            result = {
                "type": "search_results",
                "query": query,
                "chat_id": chat_id or "all_chats",
                "messages": messages,
                "count": len(messages)
            }
        else:
            if chat_id:
                # Get recent messages from specific chat
                messages = get_chat_messages(chat_id, user_id, limit)
                result = {
                    "type": "recent_messages",
                    "chat_id": chat_id,
                    "messages": messages,
                    "count": len(messages)
                }
            else:
                # Get user's chats list
                chats = get_user_chats(user_id)
                result = {
                    "type": "user_chats",
                    "chats": chats,
                    "count": len(chats)
                }
        
        log.info(f"Chat history tool used by {user_id}, returned {result.get('count', 0)} items")
        return result
        
    except Exception as e:
        log.error(f"Chat history tool error: {e}")
        return {"error": f"Failed to fetch chat history: {str(e)}"}
