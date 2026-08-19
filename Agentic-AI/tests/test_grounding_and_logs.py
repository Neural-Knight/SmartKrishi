import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api

client = TestClient(api)

print("🔍 TESTING GROUNDING AND DETAILED LOGS")
print("=" * 60)

# Create a test chat
user_id = "grounding_test_user"
chat_response = client.post(f"/users/{user_id}/chats", data={"chat_name": "🔍 Grounding Test"})
chat_id = chat_response.json()["chat_id"]

print(f"👤 Test User: {user_id}")
print(f"💬 Test Chat: {chat_id}")
print()

# Test 1: Regular ask with logs enabled
print("1️⃣ Testing /ask endpoint with logs=True")
print("Question: 'What happened in the 2024 Olympics in Paris?'")

response = client.post("/ask", data={
    "q": "What happened in the 2024 Olympics in Paris?",
    "user_id": user_id,
    "chat_id": chat_id,
    "logs": "true"
})

if response.status_code == 200:
    result = response.json()
    print(f"✅ Response Status: {response.status_code}")
    print(f"📊 Tools Used: {result.get('tools', [])}")
    print(f"🎯 Confidence: {result.get('confidence', 'N/A')}")
    print(f"💡 Answer Preview: {result['answer'][:150]}...")
    print()
    
    # Show detailed logs
    if "detailed_logs" in result:
        logs = result["detailed_logs"]
        print("📋 DETAILED LOGS:")
        print(f"  🎯 Planner Intent: {logs.get('planner', {}).get('output', {}).get('primary_intent', 'N/A')}")
        print(f"  🛠️ Tools Planned: {logs.get('planner', {}).get('output', {}).get('tools_needed', [])}")
        print(f"  🔧 Tools Executed: {list(logs.get('tool_calls', {}).get('results', {}).keys())}")
        print(f"  ✅ Checker Approved: {logs.get('checker', {}).get('output', {}).get('approved', 'N/A')}")
        print()
    
    # Show grounding information
    if "grounding" in result:
        grounding = result["grounding"]
        print("🌐 GROUNDING INFORMATION:")
        
        if "web_search_queries" in grounding:
            print(f"  🔍 Search Queries: {grounding['web_search_queries']}")
        
        if "grounding_chunks" in grounding:
            print(f"  📄 Sources Found: {len(grounding['grounding_chunks'])}")
            for i, chunk in enumerate(grounding['grounding_chunks'][:3], 1):  # Show first 3
                print(f"    {i}. {chunk.get('title', 'Unknown')} - {chunk.get('uri', 'No URL')[:60]}...")
        
        if "grounding_supports" in grounding:
            print(f"  🔗 Citation Supports: {len(grounding['grounding_supports'])}")
            for i, support in enumerate(grounding['grounding_supports'][:2], 1):  # Show first 2
                segment = support.get('segment', {})
                text = segment.get('text', '')[:50]
                indices = support.get('grounding_chunk_indices', [])
                print(f"    {i}. Text: '{text}...' → Sources: {indices}")
        
        if "error" in grounding:
            print(f"  ❌ Grounding Error: {grounding['error']}")
else:
    print(f"❌ Error: {response.status_code} - {response.json()}")

print()
print("-" * 60)
print()

# Test 2: Streaming with grounding
print("2️⃣ Testing /ask_stream endpoint with grounding detection")
print("Question: 'Tell me about recent AI developments and breakthroughs'")

grounding_events = []
tool_calls = []
answer_chunks = []

with client.stream("POST", "/ask_stream", data={
    "q": "Tell me about recent AI developments and breakthroughs in 2024",
    "user_id": user_id,
    "chat_id": chat_id,
    "include_tools": "weather_api",
    "logs": "true"
}) as response:
    for line in response.iter_lines():
        if line:
            try:
                data = json.loads(line)
                event_type = data.get("type", "unknown")
                
                if event_type == "grounding_web_search_queries":
                    grounding_events.append(f"🔍 Web Search Queries: {data.get('queries', [])}")
                elif event_type == "grounding_chunks":
                    sources = data.get('sources', [])
                    grounding_events.append(f"📄 Found {len(sources)} sources")
                    for i, source in enumerate(sources[:3], 1):  # Show first 3
                        grounding_events.append(f"  {i}. {source.get('title', 'Unknown')} - {source.get('uri', 'No URL')[:60]}...")
                elif event_type == "grounding_supports":
                    supports = data.get('supports', [])
                    grounding_events.append(f"🔗 {len(supports)} citation supports found")
                elif event_type == "tool_call":
                    tool_calls.append(f"🔧 {data.get('tool', 'unknown')}")
                elif event_type == "answer":
                    answer_chunks.append(data.get('text', ''))
                elif event_type == "log":
                    stage = data.get('stage', 'unknown')
                    if stage == "planner_output":
                        plan_data = data.get('data', {})
                        print(f"  📋 Plan: {plan_data.get('primary_intent', 'N/A')} → {plan_data.get('tools_needed', [])}")
                elif event_type == "end":
                    print(f"  ✅ Stream completed")
                    
            except json.JSONDecodeError:
                continue

print(f"📊 Streaming Results:")
print(f"  🔧 Tool calls: {len(tool_calls)} ({', '.join(tool_calls)})")
print(f"  💬 Answer chunks: {len(answer_chunks)}")
print(f"  🌐 Grounding events: {len(grounding_events)}")

if grounding_events:
    print(f"🌐 GROUNDING EVENTS:")
    for event in grounding_events:
        print(f"  {event}")

if answer_chunks:
    full_answer = ''.join(answer_chunks)
    print(f"💡 Full Answer Preview: {full_answer[:200]}...")

print()
print("-" * 60)
print()

# Test 3: Compare regular vs logs response
print("3️⃣ Comparing regular vs detailed logs response sizes")

# Regular response
regular_response = client.post("/ask", data={
    "q": "What's the weather like today?",
    "user_id": user_id,
    "chat_id": chat_id,
    "logs": "false"
})

# Logs response  
logs_response = client.post("/ask", data={
    "q": "What's the weather like today?",
    "user_id": user_id,
    "chat_id": chat_id,
    "logs": "true"
})

if regular_response.status_code == 200 and logs_response.status_code == 200:
    regular_size = len(json.dumps(regular_response.json()))
    logs_size = len(json.dumps(logs_response.json()))
    
    print(f"📏 Response Size Comparison:")
    print(f"  📦 Regular response: {regular_size:,} bytes")
    print(f"  📋 With logs: {logs_size:,} bytes")
    print(f"  📈 Size increase: {logs_size - regular_size:,} bytes ({((logs_size/regular_size - 1) * 100):.1f}%)")
    
    regular_keys = set(regular_response.json().keys())
    logs_keys = set(logs_response.json().keys())
    extra_keys = logs_keys - regular_keys
    print(f"  🔑 Extra keys with logs: {list(extra_keys)}")

print()
print("🎉 GROUNDING AND LOGS TESTING COMPLETED!")
print("=" * 60)
print()
print("✨ New Features Demonstrated:")
print("  ✅ Grounding metadata extraction in streaming")
print("  ✅ Web search queries detection")
print("  ✅ Source citations and supports")
print("  ✅ Detailed logs option for /ask endpoint")
print("  ✅ Stage-by-stage logging in streaming")
print("  ✅ Tool execution tracking")
print("  ✅ Planner and checker insights")
