import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api

client = TestClient(api)

print("🚀 COMPREHENSIVE CHAT FEATURES DEMO")
print("=" * 60)

user_id = "demo_user"

print(f"👤 Demo User: {user_id}")
print()

# Step 1: Create multiple chats
print("📁 Step 1: Creating multiple chats")
chat1 = client.post(f"/users/{user_id}/chats", data={"chat_name": "🌦️ Weather Planning"}).json()
chat2 = client.post(f"/users/{user_id}/chats", data={"chat_name": "🌱 Agriculture Advice"}).json()
chat3 = client.post(f"/users/{user_id}/chats", data={"chat_name": "📊 Market Analysis"}).json()

print(f"Created 3 chats:")
print(f"  🌦️ Weather Planning: {chat1['chat_id']}")
print(f"  🌱 Agriculture Advice: {chat2['chat_id']}")
print(f"  📊 Market Analysis: {chat3['chat_id']}")
print()

# Step 2: Have conversations in different chats
print("💬 Step 2: Having conversations in different chats")

# Weather chat
print("🌦️ Weather Planning Chat:")
questions_weather = [
    "What's the weather like in California today?",
    "Should I plan outdoor farming activities this week?",
    "How about the weather in Texas for next week?"
]

for q in questions_weather:
    print(f"  Q: {q}")
    response = client.post("/ask", data={
        "q": q,
        "user_id": user_id,
        "chat_id": chat1["chat_id"]
    })
    if response.status_code == 200:
        answer = response.json()["answer"][:100] + "..."
        print(f"  A: {answer}")
    print()

# Agriculture chat
print("🌱 Agriculture Advice Chat:")
questions_agriculture = [
    "What are the best soil conditions for growing corn?",
    "When should I plant tomatoes in the spring?"
]

for q in questions_agriculture:
    print(f"  Q: {q}")
    response = client.post("/ask", data={
        "q": q,
        "user_id": user_id,
        "chat_id": chat2["chat_id"]
    })
    if response.status_code == 200:
        answer = response.json()["answer"][:100] + "..."
        print(f"  A: {answer}")
    print()

# Step 3: Demonstrate chat history access
print("📚 Step 3: Demonstrating chat history access")

# Show message counts
for chat_name, chat_info in [("Weather", chat1), ("Agriculture", chat2), ("Market", chat3)]:
    messages = client.get(f"/users/{user_id}/chats/{chat_info['chat_id']}/messages").json()
    print(f"  {chat_name} Chat: {len(messages['messages'])} messages")

print()

# Step 4: Cross-chat analysis using chat history tool
print("🔍 Step 4: Cross-chat analysis")
print("Asking: 'Based on all my previous conversations about weather and farming, what's your recommendation?'")

response = client.post("/ask", data={
    "q": "Based on all my previous conversations about weather and farming, what's your overall recommendation for agricultural planning? Please check my chat history.",
    "user_id": user_id,
    "chat_id": chat2["chat_id"],  # Ask in agriculture chat
})

if response.status_code == 200:
    result = response.json()
    print(f"✅ Response generated successfully")
    print(f"📊 Tools used: {result['tools']}")
    print(f"💡 Answer preview: {result['answer'][:200]}...")
    print(f"🎯 Confidence: {result['confidence']}")
else:
    print(f"❌ Error: {response.json()}")
print()

# Step 5: List all chats and their status
print("📋 Step 5: Final chat overview")
all_chats = client.get(f"/users/{user_id}/chats").json()["chats"]
print(f"Total chats for {user_id}: {len(all_chats)}")

for chat in all_chats[:3]:  # Show first 3
    messages = client.get(f"/users/{user_id}/chats/{chat['chat_id']}/messages").json()
    print(f"  📁 {chat['chat_name']}")
    print(f"     💬 Messages: {len(messages['messages'])}")
    print(f"     🕒 Last updated: {chat['updated_at']}")
    print(f"     🆔 ID: {chat['chat_id']}")
    print()

# Step 6: Test streaming with chat context
print("🌊 Step 6: Testing streaming with chat context")
print("Streaming question: 'Give me a weather update for my farming locations'")

with client.stream("POST", "/ask_stream", data={
    "q": "Give me a weather update for my farming locations based on our previous discussions",
    "user_id": user_id,
    "chat_id": chat1["chat_id"],
    "include_tools": "weather_api,chat_history"
}) as response:
    tool_calls = 0
    answer_chunks = 0
    
    for line in response.iter_lines():
        if line:
            try:
                data = json.loads(line)
                if data["type"] == "tool_call":
                    tool_calls += 1
                    print(f"  🔧 Tool called: {data.get('tool', 'unknown')}")
                elif data["type"] == "answer":
                    answer_chunks += 1
                    if answer_chunks <= 3:  # Show first 3 chunks
                        print(f"  💬 Response chunk {answer_chunks}: {data['text'][:80]}...")
                elif data["type"] == "end":
                    print(f"  ✅ Stream completed")
            except:
                continue
    
    print(f"  📊 Total tool calls: {tool_calls}")
    print(f"  📝 Total answer chunks: {answer_chunks}")

print()

# Step 7: Cleanup demonstration
print("🧹 Step 7: Cleanup demonstration")
print(f"Deleting Market Analysis chat...")

delete_response = client.delete(f"/users/{user_id}/chats/{chat3['chat_id']}")
if delete_response.status_code == 200:
    print("✅ Chat deleted successfully")
else:
    print(f"❌ Delete failed: {delete_response.json()}")

# Final count
final_chats = client.get(f"/users/{user_id}/chats").json()["chats"]
print(f"📊 Final chat count: {len(final_chats)} chats remaining")

print()
print("🎉 CHAT FEATURES DEMO COMPLETED!")
print("=" * 60)
print()
print("✨ Summary of implemented features:")
print("  ✅ Multi-chat support per user")
print("  ✅ Chat-specific message history")
print("  ✅ Cross-chat history access via tool")
print("  ✅ Streaming responses with chat context")
print("  ✅ Chat management (create, list, delete)")
print("  ✅ Message persistence and retrieval")
print("  ✅ Tool integration with chat context")
print("  ✅ Backward compatibility with existing endpoints")
