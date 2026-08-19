import sqlite3, os, logging, json, uuid
from typing import List, Dict, Optional
from datetime import datetime

log = logging.getLogger("History")
DB_PATH = os.getenv("HISTORY_DB", "data/chat_history.db")
os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)

conn = sqlite3.connect(DB_PATH, check_same_thread=False)

# Create tables for chat system
conn.execute("""CREATE TABLE IF NOT EXISTS chats(
        chat_id TEXT PRIMARY KEY,
        user_id TEXT NOT NULL,
        chat_name TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)""")

conn.execute("""CREATE TABLE IF NOT EXISTS messages(
        id INTEGER PRIMARY KEY,
        chat_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        role TEXT NOT NULL,
        msg TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id))""")

# Migrate old history table if it exists
try:
    # Check if old history table exists and has data
    old_rows = conn.execute("SELECT user, role, msg, created FROM history").fetchall()
    if old_rows:
        log.info(f"Migrating {len(old_rows)} messages from old history table")
        for user, role, msg, created in old_rows:
            # Create a default chat for each user
            chat_id = f"default_{user}_{uuid.uuid4().hex[:8]}"
            conn.execute("""INSERT OR IGNORE INTO chats(chat_id, user_id, chat_name) 
                           VALUES(?, ?, ?)""", (chat_id, user, "Default Chat"))
            conn.execute("""INSERT INTO messages(chat_id, user_id, role, msg, created_at) 
                           VALUES(?, ?, ?, ?, ?)""", (chat_id, user, role, msg, created))
        conn.commit()
        log.info("Migration completed")
except sqlite3.OperationalError:
    # Old table doesn't exist, that's fine
    pass

conn.commit()

# Chat management functions
def create_chat(user_id: str, chat_name: str) -> str:
    """Create a new chat and return chat_id"""
    chat_id = f"chat_{uuid.uuid4().hex}"
    conn.execute("""INSERT INTO chats(chat_id, user_id, chat_name) VALUES(?, ?, ?)""",
                 (chat_id, user_id, chat_name))
    conn.commit()
    log.info(f"Created chat {chat_id} for user {user_id}")
    return chat_id

def get_user_chats(user_id: str) -> List[Dict[str, str]]:
    """Get all chats for a user"""
    rows = conn.execute("""SELECT chat_id, chat_name, created_at, updated_at 
                          FROM chats WHERE user_id=? ORDER BY updated_at DESC""", 
                       (user_id,)).fetchall()
    return [{"chat_id": r[0], "chat_name": r[1], "created_at": r[2], "updated_at": r[3]} 
            for r in rows]

def get_chat_info(chat_id: str, user_id: str) -> Optional[Dict[str, str]]:
    """Get chat information"""
    row = conn.execute("""SELECT chat_id, chat_name, created_at, updated_at 
                         FROM chats WHERE chat_id=? AND user_id=?""", 
                      (chat_id, user_id)).fetchone()
    if row:
        return {"chat_id": row[0], "chat_name": row[1], "created_at": row[2], "updated_at": row[3]}
    return None

def delete_chat(chat_id: str, user_id: str) -> bool:
    """Delete a chat and all its messages"""
    # Verify ownership
    chat = get_chat_info(chat_id, user_id)
    if not chat:
        return False
    
    conn.execute("DELETE FROM messages WHERE chat_id=?", (chat_id,))
    conn.execute("DELETE FROM chats WHERE chat_id=? AND user_id=?", (chat_id, user_id))
    conn.commit()
    log.info(f"Deleted chat {chat_id} for user {user_id}")
    return True

def rename_chat(chat_id: str, user_id: str, new_name: str) -> bool:
    """Rename a chat (updates updated_at)"""
    # Verify ownership
    chat = get_chat_info(chat_id, user_id)
    if not chat:
        return False
    conn.execute("""UPDATE chats SET chat_name=?, updated_at=CURRENT_TIMESTAMP
                  WHERE chat_id=? AND user_id=?""", (new_name, chat_id, user_id))
    conn.commit()
    log.info(f"Renamed chat {chat_id} for user {user_id} to '{new_name}'")
    return True

# Message functions
def add_message(chat_id: str, user_id: str, role: str, msg: str):
    """Add a message to a specific chat"""
    conn.execute("""INSERT INTO messages(chat_id, user_id, role, msg) VALUES(?, ?, ?, ?)""",
                 (chat_id, user_id, role, msg))
    # Update chat's updated_at timestamp
    conn.execute("""UPDATE chats SET updated_at=CURRENT_TIMESTAMP WHERE chat_id=?""", (chat_id,))
    conn.commit()

def get_chat_messages(chat_id: str, user_id: str, limit: int = 100) -> List[Dict[str, str]]:
    """Get messages for a specific chat"""
    rows = conn.execute("""SELECT role, msg, created_at FROM messages 
                          WHERE chat_id=? AND user_id=? 
                          ORDER BY created_at ASC LIMIT ?""", 
                       (chat_id, user_id, limit)).fetchall()
    return [{"role": r[0], "msg": r[1], "created_at": r[2]} for r in rows]

def search_messages(user_id: str, query: str, chat_id: Optional[str] = None, k: int = 4) -> List[Dict[str, str]]:
    """Search messages across chats or within a specific chat"""
    terms = query.lower().split()
    if chat_id:
        clause = "user_id=? AND chat_id=? AND (" + " OR ".join("msg LIKE ?" for _ in terms) + ")"
        params = [user_id, chat_id] + [f"%{t}%" for t in terms] + [k]
    else:
        clause = "user_id=? AND (" + " OR ".join("msg LIKE ?" for _ in terms) + ")"
        params = [user_id] + [f"%{t}%" for t in terms] + [k]
    
    sql = (f"SELECT chat_id, role, msg, created_at FROM messages WHERE {clause} "
           "ORDER BY created_at DESC LIMIT ?")
    rows = conn.execute(sql, params).fetchall()
    return [{"chat_id": r[0], "role": r[1], "msg": r[2], "created_at": r[3]} for r in rows]

# Backward compatibility functions
def add(user: str, role: str, msg: str):
    """Backward compatibility - creates a default chat if needed"""
    # Find or create a default chat for the user
    chats = get_user_chats(user)
    default_chat = next((c for c in chats if c["chat_name"] == "Default Chat"), None)
    
    if not default_chat:
        chat_id = create_chat(user, "Default Chat")
    else:
        chat_id = default_chat["chat_id"]
    
    add_message(chat_id, user, role, msg)

def search(user: str, query: str, k: int = 4) -> List[Dict[str, str]]:
    """Backward compatibility - search in all chats"""
    results = search_messages(user, query, None, k)
    return [{"role": r["role"], "msg": r["msg"]} for r in results]

def dump_json() -> str:            # optional helper
    rows = conn.execute("SELECT user_id, role, msg FROM messages"
                        " ORDER BY id").fetchall()
    return json.dumps([{"user":u,"role":r,"msg":m} for u,r,m in rows], indent=2)
