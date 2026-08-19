#!/usr/bin/env python3
"""
Direct test of the streaming function without HTTP server
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# Load environment variables from .env file
def load_env():
    env_path = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), '.env')
    if os.path.exists(env_path):
        with open(env_path, 'r') as f:
            for line in f:
                if '=' in line and not line.strip().startswith('#'):
                    key, value = line.strip().split('=', 1)
                    os.environ[key] = value

# Load .env before importing app modules
load_env()

import asyncio
import json
from app.main import ask_stream

async def test_direct_streaming():
    """Test the streaming function directly"""
    
    print("🧪 TESTING: Direct Streaming Function")
    print("=" * 50)
    
    # Test parameters
    user_id = "test_user_pipeline"
    chat_id = "test_chat_pipeline"
    query = "I uploaded a CSV file with agricultural data. Please analyze it using code execution and calculate the total yield."
    
    print(f"📤 User: {user_id}")
    print(f"💬 Chat: {chat_id}")
    print(f"❓ Query: {query}")
    
    print("\n📥 Streaming response:")
    print("-" * 40)
    
    # Initialize counters
    chunk_count = 0
    total_chars = 0
    
    # Since ask_stream returns a StreamingResponse, let's extract the event generator
    from fastapi.responses import StreamingResponse
    
    # Call the streaming function directly
    try:
        response = await ask_stream(
            q=query,
            user_id=user_id, 
            chat_id=chat_id,
            include_tools=None,
            logs=True
        )
        
        if isinstance(response, StreamingResponse):
            print("✅ Got StreamingResponse, extracting events...")
            
            # Extract the event generator from the StreamingResponse
            async for chunk in response.body_iterator:
                # Convert bytes/memoryview to string
                if isinstance(chunk, memoryview):
                    chunk_data = chunk.tobytes().decode('utf-8')
                elif isinstance(chunk, bytes):
                    chunk_data = chunk.decode('utf-8')
                else:
                    chunk_data = str(chunk)
                    
                chunk_count += 1
                total_chars += len(chunk_data)
                
                print(f"🔍 CHUNK {chunk_count}: '{chunk_data}' (len: {len(chunk_data)})")
                
                # Parse and display the chunk
                for line in str(chunk_data).strip().split('\n'):
                    if not line.strip():
                        continue
                        
                    try:
                        parsed = json.loads(line.strip())
                        chunk_type = parsed.get('type', 'unknown')
                        
                        if chunk_type == 'log':
                            stage = parsed.get('stage', '')
                            message = parsed.get('message', '')
                            print(f"📝 LOG [{stage}]: {message}")
                            
                        elif chunk_type == 'plan':
                            plan = parsed.get('plan', {})
                            print(f"🧠 PLAN: {plan}")
                            
                        elif chunk_type == 'tool_call':
                            tool = parsed.get('tool', '')
                            result = parsed.get('result', '')
                            print(f"🔧 TOOL [{tool}]: {str(result)[:100]}...")
                            
                        elif chunk_type == 'response_chunk':
                            content = parsed.get('content', '')
                            print(f"💬 RESPONSE: {content}")
                            
                        elif chunk_type == 'code_execution':
                            stage = parsed.get('stage', '')
                            if stage == 'code':
                                code = parsed.get('code', '')
                                print(f"🔧 CODE: {code[:100]}...")
                            elif stage == 'result':
                                outcome = parsed.get('outcome', '')
                                result = parsed.get('result', '')
                                print(f"📊 CODE RESULT [{outcome}]: {result[:100]}...")
                                
                        elif chunk_type == 'response':
                            response_text = parsed.get('response', '')
                            print(f"✅ FINAL RESPONSE: {response_text[:200]}...")
                            
                        elif chunk_type == 'end':
                            print("🏁 STREAM END")
                            
                    except json.JSONDecodeError:
                        print(f"📄 RAW CHUNK: {line[:100]}...")
        else:
            print(f"❌ Unexpected response type: {type(response)}")
    
    except Exception as e:
        print(f"❌ Error during streaming: {e}")
        import traceback
        traceback.print_exc()
    
    print("\n" + "=" * 50)
    print("🎯 DIRECT STREAMING TEST RESULTS:")
    print(f"✅ Total chunks received: {chunk_count}")
    print(f"✅ Total characters: {total_chars}")
    
    if chunk_count > 0:
        print("✅ Streaming function is working correctly!")
    else:
        print("❌ No chunks received - streaming function may have issues")

if __name__ == "__main__":
    asyncio.run(test_direct_streaming())
