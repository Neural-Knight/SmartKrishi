import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api
from app.tools.chat_history import chat_history_tool

client = TestClient(api)

print("Direct Testing of Chat History Tool")
print("=" * 40)

# Create a chat with some messages
chat_response = client.post("/users/testuser/chats", data={"chat_name": "Test Chat"})
chat_id = chat_response.json()["chat_id"]

# Add some messages
client.post("/ask", data={
    "q": "What's the weather in Tokyo?",
    "user_id": "testuser",
    "chat_id": chat_id
})

client.post("/ask", data={
    "q": "How about the soil quality in Japan?", 
    "user_id": "testuser",
    "chat_id": chat_id
})

print("1) Testing chat history tool - search for 'weather'")
result = chat_history_tool(
    query="weather",
    user_id="testuser",
    chat_id=chat_id,
    limit=5
)
print(json.dumps(result, indent=2))
print()

print("2) Testing chat history tool - search across all chats")
result = chat_history_tool(
    query="Tokyo",
    user_id="testuser",
    chat_id="",
    limit=5
)
print(json.dumps(result, indent=2))
print()

print("3) Testing chat history tool - get recent messages")
result = chat_history_tool(
    query="",
    user_id="testuser", 
    chat_id=chat_id,
    limit=5
)
print(json.dumps(result, indent=2))
print()

print("4) Testing chat history tool - list user chats")
result = chat_history_tool(
    query="",
    user_id="testuser",
    chat_id="",
    limit=5
)
print(json.dumps(result, indent=2))
print()

print("Direct tool testing completed!")
