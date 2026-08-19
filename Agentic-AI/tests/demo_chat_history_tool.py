import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api

client = TestClient(api)

print("Testing Chat History Tool Integration")
print("=" * 50)

# Create a chat and have a conversation
chat_response = client.post("/users/bob/chats", data={"chat_name": "Weather Planning"})
chat_id = chat_response.json()["chat_id"]

# Ask several questions to build history
questions = [
    "What's the weather in New York?",
    "How about Paris?", 
    "Should I plan outdoor activities this weekend in London?",
    "What are the soil conditions for farming in California?"
]

print("Building conversation history...")
for i, q in enumerate(questions, 1):
    print(f"Question {i}: {q}")
    response = client.post("/ask", data={
        "q": q,
        "user_id": "bob",
        "chat_id": chat_id
    })
    print(f"  Status: {response.status_code}")
    if response.status_code == 200:
        tools_used = response.json().get("tools", [])
        print(f"  Tools used: {tools_used}")
    print()

# Now test the chat history tool by asking Gemini to use it
print("Testing Chat History Tool Usage...")
print("Asking: 'Based on our previous weather discussions, what would you recommend for travel planning?'")

# Create a new chat for this test
chat2_response = client.post("/users/bob/chats", data={"chat_name": "Travel Planning"})
chat2_id = chat2_response.json()["chat_id"]

# This should trigger Gemini to use the chat_history tool
response = client.post("/ask_stream", data={
    "q": "Based on our previous weather discussions, what would you recommend for travel planning? Please check our chat history.",
    "user_id": "bob",
    "chat_id": chat2_id,
    "include_tools": "chat_history,weather_api"
})

print("Streaming response:")
for line in response.iter_lines():
    if line:
        try:
            data = json.loads(line)
            if data["type"] == "tool_call" and "chat_history" in str(data):
                print(f"  🔍 Chat History Tool Called: {data}")
            elif data["type"] == "tool_response" and "chat_history" in str(data):
                print(f"  📜 Chat History Results: {str(data)[:200]}...")
            elif data["type"] == "answer":
                print(f"  💬 Response: {data['text'][:100]}...")
            elif data["type"] in ["prompt", "planner"]:
                print(f"  🤖 {data['type'].title()}: {str(data)[:100]}...")
        except:
            continue

print("\nDemo completed!")
