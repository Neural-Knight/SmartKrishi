#!/usr/bin/env python3
"""
Test the current streaming ask endpoint to see how code execution is handled
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import requests
import json

def test_current_streaming_endpoint():
    """Test the current /ask_stream endpoint to see how it handles code execution"""
    
    print("🌐 Testing Current /ask_stream Endpoint")
    print("=" * 50)
    
    base_url = "http://localhost:8000"
    user_id = "test_user"
    chat_id = "test_chat"
    
    # First create a chat
    create_chat_url = f"{base_url}/users/{user_id}/chats"
    create_data = {"chat_id": chat_id, "chat_name": "Test Code Execution Chat"}
    
    print(f"📤 Creating chat: {create_chat_url}")
    try:
        create_response = requests.post(create_chat_url, json=create_data)
        print(f"Create chat status: {create_response.status_code}")
        if create_response.status_code != 200:
            print(f"Create chat response: {create_response.text}")
    except Exception as e:
        print(f"⚠️ Chat creation error (continuing anyway): {e}")
    
    # Now test the streaming endpoint
    stream_url = f"{base_url}/ask_stream"
    data = {
        "user_id": user_id,
        "q": "Calculate the average of these corn yields: 150, 165, 142, 178, 156 bushels per acre. Use Python code to calculate mean and standard deviation.",
        "chat_id": chat_id,
        "include_tools": ""
    }
    
    print(f"📤 Request to: {stream_url}")
    print(f"📤 Data: {data}")
    print("\n📥 Streaming response:")
    print("-" * 30)
    
    try:
        response = requests.post(stream_url, data=data, stream=True)
        print(f"Status Code: {response.status_code}")
        
        if response.status_code != 200:
            print(f"❌ Error: {response.text}")
            return
        
        event_count = 0
        for line in response.iter_lines():
            if line:
                event_count += 1
                line_str = line.decode('utf-8')
                print(f"📥 Event {event_count}: {line_str}")
                
                try:
                    data = json.loads(line_str)
                    if data.get('type') == 'code_execution':
                        print(f"   🔥 CODE EXECUTION EVENT DETECTED!")
                        print(f"   Stage: {data.get('stage', 'unknown')}")
                        if 'code' in data:
                            print(f"   Code: {data['code'][:100]}...")
                        if 'result' in data:
                            print(f"   Result: {data['result'][:100]}...")
                except json.JSONDecodeError:
                    print(f"   (Non-JSON content)")
        
        print(f"\n📈 Total events received: {event_count}")
        
    except requests.exceptions.ConnectionError:
        print("❌ Cannot connect to localhost:8000")
        print("   Please start the server first with: python -m app.main")
    except Exception as e:
        print(f"❌ Error: {e}")

if __name__ == "__main__":
    test_current_streaming_endpoint()
