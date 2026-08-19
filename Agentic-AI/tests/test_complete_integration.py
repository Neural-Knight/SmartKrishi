#!/usr/bin/env python3
"""
Complete integration test: Create chat, upload file, test streaming
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
import tempfile
import csv
from app.main import ask_stream
from app.history import create_chat
from app.client_manager import client_manager

async def test_complete_integration():
    """Test complete file upload and streaming integration"""
    
    print("🧪 TESTING: Complete Integration - Chat + File + Streaming")
    print("=" * 60)
    
    # Test parameters
    user_id = "test_integration_user"
    
    # Step 1: Create a chat
    print("1️⃣ Creating chat...")
    chat_id = create_chat(user_id, "Test file analysis chat")
    print(f"✅ Chat created: {chat_id}")
    
    # Step 2: Create and upload a CSV file
    print("\n2️⃣ Creating and uploading CSV file...")
    
    # Create test CSV
    csv_file = "test_integration_data.csv"
    with open(csv_file, 'w', newline='') as f:
        f.write('Product,Yield_Tons,Price_Per_Ton\n')
        f.write('Wheat,150.5,250\n')
        f.write('Corn,200.3,180\n')
        f.write('Barley,75.2,300\n')
        f.write('Rice,120.8,400\n')
    
    try:
        # Upload file using the proper app pipeline
        from app.media import save_file
        
        # Read file content
        with open(csv_file, 'rb') as f:
            file_content = f.read()
        
        # Use the app's file upload pipeline
        upload_result = save_file(
            user=user_id,
            content=file_content,
            filename="agricultural_data.csv",
            kind="csv",
            chat_id=chat_id
        )
        
        if upload_result.get("file_id"):
            print(f"✅ File uploaded with ID: {upload_result['file_id']}")
            print(f"📁 Status: {upload_result.get('processing_status', 'unknown')}")
            
            # Wait a bit for processing
            import time
            print("⏳ Waiting for file processing...")
            time.sleep(3)
            
        else:
            print(f"❌ File upload failed: {upload_result}")
            return
        
        # Step 3: Test streaming with the uploaded file
        print("\n3️⃣ Testing streaming with uploaded file...")
        
        query = "I uploaded a CSV file with agricultural data. Please analyze it using code execution and calculate the total yield for all crops combined."
        
        print(f"📤 User: {user_id}")
        print(f"💬 Chat: {chat_id}")
        print(f"❓ Query: {query}")
        
        print("\n📥 Streaming response:")
        print("-" * 50)
        
        # Initialize counters
        chunk_count = 0
        total_chars = 0
        response_chunks = []
        code_executions = []
        
        # Call the streaming function
        try:
            response = await ask_stream(
                q=query,
                user_id=user_id, 
                chat_id=chat_id,
                include_tools=None,
                logs=True
            )
            
            from fastapi.responses import StreamingResponse
            if isinstance(response, StreamingResponse):
                print("✅ Got StreamingResponse, processing events...")
                
                async for chunk in response.body_iterator:
                    # Convert to string
                    if isinstance(chunk, memoryview):
                        chunk_data = chunk.tobytes().decode('utf-8')
                    elif isinstance(chunk, bytes):
                        chunk_data = chunk.decode('utf-8')
                    else:
                        chunk_data = str(chunk)
                        
                    chunk_count += 1
                    total_chars += len(chunk_data)
                    
                    # Parse and display the chunk
                    for line in str(chunk_data).strip().split('\n'):
                        if not line.strip():
                            continue
                            
                        try:
                            parsed = json.loads(line.strip())
                            chunk_type = parsed.get('type', 'unknown')
                            
                            if chunk_type == 'error':
                                message = parsed.get('message', '')
                                print(f"❌ ERROR: {message}")
                                
                            elif chunk_type == 'log':
                                stage = parsed.get('stage', '')
                                message = parsed.get('message', '')
                                print(f"📝 LOG [{stage}]: {message}")
                                
                            elif chunk_type == 'plan':
                                plan = parsed.get('plan', {})
                                tools = plan.get('tools_needed', [])
                                intent = plan.get('primary_intent', '')
                                print(f"🧠 PLAN: Intent='{intent}', Tools={tools}")
                                
                            elif chunk_type == 'tool_call':
                                tool = parsed.get('tool', '')
                                result = parsed.get('result', '')
                                print(f"🔧 TOOL [{tool}]: {str(result)[:150]}...")
                                
                            elif chunk_type == 'response_chunk':
                                content = parsed.get('content', '')
                                response_chunks.append(content)
                                print(f"💬 RESPONSE: {content[:100]}...")
                                
                            elif chunk_type == 'code_execution':
                                stage = parsed.get('stage', '')
                                if stage == 'code':
                                    code = parsed.get('code', '')
                                    language = parsed.get('language', 'unknown')
                                    print(f"🔧 CODE [{language}]: {code[:150]}...")
                                elif stage == 'result':
                                    outcome = parsed.get('outcome', '')
                                    result = parsed.get('result', '')
                                    code_executions.append(result)
                                    print(f"📊 CODE RESULT [{outcome}]: {result[:150]}...")
                                    
                            elif chunk_type == 'response':
                                response_text = parsed.get('response', '')
                                print(f"✅ FINAL RESPONSE: {response_text[:300]}...")
                                
                            elif chunk_type == 'end':
                                print("🏁 STREAM END")
                                
                        except json.JSONDecodeError:
                            print(f"📄 RAW: {line[:100]}...")
            else:
                print(f"❌ Unexpected response type: {type(response)}")
        
        except Exception as e:
            print(f"❌ Error during streaming: {e}")
            import traceback
            traceback.print_exc()
        
        # Step 4: Results summary
        print("\n" + "=" * 60)
        print("🎯 INTEGRATION TEST RESULTS:")
        print(f"✅ Chat created: {chat_id}")
        print(f"✅ File uploaded: {upload_result.get('success', False)}")
        print(f"✅ Total chunks received: {chunk_count}")
        print(f"✅ Total characters: {total_chars}")
        print(f"✅ Response chunks: {len(response_chunks)}")
        print(f"✅ Code executions: {len(code_executions)}")
        
        # Check for specific success indicators
        full_response = ''.join(response_chunks)
        has_calculation = any('yield' in str(result).lower() or 'total' in str(result).lower() for result in code_executions)
        
        if chunk_count > 5:
            print("✅ Streaming is working correctly!")
        else:
            print("⚠️  Limited streaming response")
            
        if has_calculation:
            print("✅ Code execution with calculations detected!")
        else:
            print("⚠️  No clear calculation results found")
            
        if len(response_chunks) > 0:
            print("✅ AI response generated successfully!")
        else:
            print("⚠️  No AI response received")
    
    finally:
        # Cleanup
        try:
            os.unlink(csv_file)
            print(f"\n🧹 Cleaned up: {csv_file}")
        except:
            pass

if __name__ == "__main__":
    asyncio.run(test_complete_integration())
