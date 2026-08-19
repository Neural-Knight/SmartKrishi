import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api

client = TestClient(api)

print("Testing Chat Feature Endpoints")
print("=" * 50)

# Test 1: Create a new chat
print("1) Creating a new chat")
response = client.post("/users/alice/chats", data={"chat_name": "Weather Discussions"})
print(f"Status: {response.status_code}")
chat_response = response.json()
print(f"Response: {json.dumps(chat_response, indent=2)}")
chat_id = chat_response.get("chat_id")
print()

# Test 2: List user chats
print("2) Listing user chats")
response = client.get("/users/alice/chats")
print(f"Status: {response.status_code}")
print(f"Response: {json.dumps(response.json(), indent=2)}")
print()

# Test 3: Get chat details
print("3) Getting chat details")
response = client.get(f"/users/alice/chats/{chat_id}")
print(f"Status: {response.status_code}")
print(f"Response: {json.dumps(response.json(), indent=2)}")
print()

# Test 4: Ask a question in the chat
print("4) Asking a question in the chat")
response = client.post("/ask", data={
    "q": "What's the weather like in New York?",
    "user_id": "alice",
    "chat_id": chat_id
})
print(f"Status: {response.status_code}")
if response.status_code == 200:
    result = response.json()
    print(f"Answer: {result['answer'][:200]}...")
    print(f"Tools used: {result['tools']}")
    print(f"Chat ID: {result['chat_id']}")
else:
    print(f"Error: {response.json()}")
print()

# Test 5: Get chat messages
print("5) Getting chat messages")
response = client.get(f"/users/alice/chats/{chat_id}/messages")
print(f"Status: {response.status_code}")
messages = response.json()
print(f"Number of messages: {len(messages['messages'])}")
for i, msg in enumerate(messages['messages'][-3:], 1):  # Show last 3 messages
    print(f"  Message {i}: {msg['role']} - {msg['msg'][:100]}...")
print()

# Test 6: Create another chat
print("6) Creating another chat")
response = client.post("/users/alice/chats", data={"chat_name": "Agricultural Advice"})
chat2_id = response.json().get("chat_id")
print(f"Created chat 2: {chat2_id}")

# Ask a question in the second chat
response = client.post("/ask", data={
    "q": "What soil conditions are best for growing tomatoes?",
    "user_id": "alice", 
    "chat_id": chat2_id
})
print(f"Asked question in chat 2, status: {response.status_code}")
print()

# Test 7: List all chats again
print("7) Listing all chats")
response = client.get("/users/alice/chats")
chats = response.json()["chats"]
print(f"Total chats: {len(chats)}")
for chat in chats:
    print(f"  - {chat['chat_name']} (ID: {chat['chat_id']})")
print()

# Test 8: Test streaming with chat
print("8) Testing streaming with chat")
print("Streaming response for: 'Give me a weather summary for Paris'")
with client.stream("POST", "/ask_stream", data={
    "q": "Give me a weather summary for Paris",
    "user_id": "alice",
    "chat_id": chat_id,
    "include_tools": "weather_api"
}) as response:
    lines_count = 0
    for line in response.iter_lines():
        if line:
            lines_count += 1
            if lines_count <= 5:  # Show first 5 lines
                try:
                    data = json.loads(line)
                    text_content = data.get('text', str(data))[:100]
                    print(f"  {data['type']}: {text_content}...")
                except:
                    print(f"  Raw: {line[:100]}...")
    print(f"  Total response lines: {lines_count}")
print()

# Test 9: Delete a chat
print("9) Deleting a chat")
response = client.delete(f"/users/alice/chats/{chat2_id}")
print(f"Delete status: {response.status_code}")
print(f"Response: {response.json()}")

# Verify deletion
response = client.get("/users/alice/chats")
chats = response.json()["chats"]
print(f"Remaining chats: {len(chats)}")
print()

print("Chat feature testing completed!")
