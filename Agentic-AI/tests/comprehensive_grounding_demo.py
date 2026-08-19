import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api
from grounding_utils import add_citations_to_text, extract_grounding_info, create_citation_summary

client = TestClient(api)

print("🌟 COMPREHENSIVE GROUNDING & CHAT FEATURES DEMO")
print("=" * 70)

# Setup
user_id = "comprehensive_demo_user"
chat_response = client.post(f"/users/{user_id}/chats", data={"chat_name": "🌟 Comprehensive Demo"})
chat_id = chat_response.json()["chat_id"]

print(f"👤 Demo User: {user_id}")
print(f"💬 Demo Chat: {chat_id}")
print()

# Demo 1: Chat with grounding and citations
print("1️⃣ CHAT WITH GOOGLE GROUNDING & CITATIONS")
print("-" * 50)

question = "What are the latest developments in renewable energy technology in 2024?"
print(f"❓ Question: {question}")

response = client.post("/ask", data={
    "q": question,
    "user_id": user_id,
    "chat_id": chat_id,
    "logs": "true"
})

if response.status_code == 200:
    result = response.json()
    
    print(f"✅ Response generated successfully")
    print(f"🎯 Confidence: {result['confidence']}")
    print(f"🛠️ Tools used: {result['tools']}")
    
    # Extract and display grounding information
    grounding_info = extract_grounding_info(result)
    summary = create_citation_summary(grounding_info)
    print(f"🌐 {summary}")
    
    # Show original answer
    original_answer = result['answer'][:300] + "..." if len(result['answer']) > 300 else result['answer']
    print(f"\n📝 Original Answer (first 300 chars):")
    print(f"   {original_answer}")
    
    # Add citations if grounding data is available
    if grounding_info.get('has_grounding') and 'grounding' in result:
        grounding = result['grounding']
        if 'grounding_supports' in grounding and 'grounding_chunks' in grounding:
            cited_answer = add_citations_to_text(
                result['answer'], 
                grounding['grounding_supports'], 
                grounding['grounding_chunks']
            )
            cited_preview = cited_answer[:400] + "..." if len(cited_answer) > 400 else cited_answer
            print(f"\n📖 Answer with Citations (first 400 chars):")
            print(f"   {cited_preview}")
            
            # Show citation details
            chunks = grounding['grounding_chunks']
            print(f"\n🔗 Source Details:")
            for i, chunk in enumerate(chunks[:5], 1):  # Show first 5 sources
                title = chunk.get('title', 'Unknown')
                uri = chunk.get('uri', 'No URL')
                print(f"   [{i}] {title}")
                print(f"       {uri}")
                print()

print()
print("=" * 70)
print()

# Demo 2: Streaming with real-time grounding events
print("2️⃣ STREAMING WITH REAL-TIME GROUNDING EVENTS")
print("-" * 50)

question2 = "What were the major space exploration achievements in 2024?"
print(f"❓ Question: {question2}")
print(f"🌊 Streaming response:")

grounding_queries = []
grounding_sources = []
citations = []
answer_parts = []

with client.stream("POST", "/ask_stream", data={
    "q": question2,
    "user_id": user_id,
    "chat_id": chat_id,
    "logs": "true"
}) as response:
    print()
    for line in response.iter_lines():
        if line:
            try:
                data = json.loads(line)
                event_type = data.get("type", "unknown")
                
                if event_type == "grounding_web_search_queries":
                    queries = data.get('queries', [])
                    grounding_queries.extend(queries)
                    print(f"   🔍 Web Search: {', '.join(queries[:2])}..." if len(queries) > 2 else f"   🔍 Web Search: {', '.join(queries)}")
                    
                elif event_type == "grounding_chunks":
                    sources = data.get('sources', [])
                    grounding_sources.extend(sources)
                    print(f"   📄 Found {len(sources)} sources from web search")
                    
                elif event_type == "grounding_supports":
                    supports = data.get('supports', [])
                    citations.extend(supports)
                    print(f"   🔗 Generated {len(supports)} citation supports")
                    
                elif event_type == "answer":
                    text = data.get('text', '')
                    answer_parts.append(text)
                    if len(answer_parts) % 10 == 0:  # Show progress every 10 chunks
                        print(f"   💬 Generating response... ({len(answer_parts)} chunks)")
                        
                elif event_type == "end":
                    print(f"   ✅ Streaming completed")
                    
            except json.JSONDecodeError:
                continue

# Summarize streaming results
full_answer = ''.join(answer_parts)
print(f"\n📊 Streaming Summary:")
print(f"   🔍 Search queries: {len(grounding_queries)}")
print(f"   📄 Sources found: {len(grounding_sources)}")
print(f"   🔗 Citations: {len(citations)}")
print(f"   💬 Answer length: {len(full_answer):,} characters")

if grounding_sources:
    print(f"\n🌐 Top Sources:")
    for i, source in enumerate(grounding_sources[:3], 1):
        print(f"   {i}. {source.get('title', 'Unknown')} - {source.get('uri', 'No URL')[:60]}...")

print()
print("=" * 70)
print()

# Demo 3: Chat history with grounding context
print("3️⃣ CHAT HISTORY WITH GROUNDING CONTEXT")
print("-" * 50)

# Ask a follow-up question that should reference previous conversation
followup_question = "Based on our previous discussions, what should I focus on for my research project?"
print(f"❓ Follow-up Question: {followup_question}")

response3 = client.post("/ask", data={
    "q": followup_question,
    "user_id": user_id,
    "chat_id": chat_id,
    "logs": "true"
})

if response3.status_code == 200:
    result3 = response3.json()
    
    print(f"✅ Response Status: {response3.status_code}")
    print(f"🛠️ Tools used: {result3['tools']}")
    
    # Check if chat history was used
    if 'detailed_logs' in result3:
        logs = result3['detailed_logs']
        if 'tool_calls' in logs and 'results' in logs['tool_calls']:
            tool_results = logs['tool_calls']['results']
            if 'chat_history' in tool_results:
                print(f"📚 Chat history was accessed for context")
                history_result = tool_results['chat_history']
                if isinstance(history_result, dict) and 'count' in history_result:
                    print(f"   📊 Retrieved {history_result['count']} relevant messages")
    
    # Show grounding for this response
    grounding_info3 = extract_grounding_info(result3)
    if grounding_info3.get('has_grounding'):
        summary3 = create_citation_summary(grounding_info3)
        print(f"🌐 {summary3}")
    
    answer_preview = result3['answer'][:250] + "..." if len(result3['answer']) > 250 else result3['answer']
    print(f"\n💡 Response Preview:")
    print(f"   {answer_preview}")

print()
print("=" * 70)
print()

# Demo 4: Chat management summary
print("4️⃣ CHAT SESSION SUMMARY")
print("-" * 50)

# Get chat messages
messages_response = client.get(f"/users/{user_id}/chats/{chat_id}/messages")
if messages_response.status_code == 200:
    messages = messages_response.json()['messages']
    
    print(f"📊 Chat Statistics:")
    print(f"   💬 Total messages: {len(messages)}")
    
    user_messages = [msg for msg in messages if msg['role'] == 'user']
    assistant_messages = [msg for msg in messages if msg['role'] == 'assistant']
    
    print(f"   👤 User messages: {len(user_messages)}")
    print(f"   🤖 Assistant messages: {len(assistant_messages)}")
    
    total_chars = sum(len(msg['msg']) for msg in assistant_messages)
    print(f"   📝 Total assistant response length: {total_chars:,} characters")
    
    print(f"\n📋 Conversation Flow:")
    for i, msg in enumerate(messages[-6:], 1):  # Show last 6 messages
        role_icon = "👤" if msg['role'] == 'user' else "🤖"
        preview = msg['msg'][:80] + "..." if len(msg['msg']) > 80 else msg['msg']
        print(f"   {i}. {role_icon} {preview}")

print()
print("🎉 COMPREHENSIVE DEMO COMPLETED!")
print("=" * 70)
print()
print("✨ Features Demonstrated:")
print("  ✅ Google Search grounding with web queries")
print("  ✅ Real-time source detection in streaming")
print("  ✅ Citation support extraction")
print("  ✅ Inline citation generation")
print("  ✅ Detailed execution logs")
print("  ✅ Chat history integration")
print("  ✅ Multi-chat conversation management")
print("  ✅ Tool execution tracking")
print("  ✅ Response quality metrics")
print("  ✅ Source verification and attribution")
