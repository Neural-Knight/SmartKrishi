#!/usr/bin/env python3
"""
Test Gemini Code Execution - Direct API Test
Based on the user's example to understand how code execution works.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from google import genai
from google.genai import types

# Load environment variables
from dotenv import load_dotenv
load_dotenv()

def test_code_execution_direct():
    """Test code execution using the direct chat API as shown by user"""
    
    print("🧪 Testing Direct Code Execution API")
    print("=" * 50)
    
    # Read API key from .env file
    env_path = os.path.join(os.path.dirname(os.path.dirname(__file__)), '.env')
    api_key = None
    
    if os.path.exists(env_path):
        with open(env_path, 'r') as f:
            for line in f:
                if line.startswith('GOOGLE_API_KEY='):
                    api_key = line.split('=', 1)[1].strip()
                    break
    
    if not api_key:
        print("❌ No GOOGLE_API_KEY found in .env file")
        return
    
    print(f"✅ Using API key: {api_key[:10]}...")
    
    client = genai.Client(api_key=api_key)

    chat = client.chats.create(
        model="gemini-2.5-flash",
        config=types.GenerateContentConfig(
            tools=[types.Tool(code_execution=types.ToolCodeExecution())]
        ),
    )

    print("📝 Sending initial message...")
    response = chat.send_message("I have a math question for you.")
    print(f"Response: {response.text}")
    print()

    print("🔢 Asking for prime numbers calculation...")
    response = chat.send_message(
        "What is the sum of the first 50 prime numbers? "
        "Generate and run code for the calculation, and make sure you get all 50."
    )

    print("📊 Response parts:")
    print("-" * 30)
    
    if response.candidates and len(response.candidates) > 0:
        candidate = response.candidates[0]
        if candidate.content and candidate.content.parts:
            for i, part in enumerate(candidate.content.parts):
                print(f"Part {i+1}:")
                if part.text is not None:
                    print(f"  📝 Text: {part.text}")
                if part.executable_code is not None:
                    print(f"  💻 Code:")
                    print(f"     Language: {part.executable_code.language}")
                    print(f"     Code: {part.executable_code.code}")
                if part.code_execution_result is not None:
                    print(f"  ✅ Result:")
                    print(f"     Outcome: {part.code_execution_result.outcome}")
                    print(f"     Output: {part.code_execution_result.output}")
                print("-" * 30)
        else:
            print("No content parts found")
    else:
        print("No candidates found")

def test_streaming_code_execution():
    """Test code execution with streaming to see how events come through"""
    
    print("\n🌊 Testing Streaming Code Execution")
    print("=" * 50)
    
    # Read API key from .env file
    env_path = os.path.join(os.path.dirname(os.path.dirname(__file__)), '.env')
    api_key = None
    
    if os.path.exists(env_path):
        with open(env_path, 'r') as f:
            for line in f:
                if line.startswith('GOOGLE_API_KEY='):
                    api_key = line.split('=', 1)[1].strip()
                    break
    
    if not api_key:
        print("❌ No GOOGLE_API_KEY found in .env file")
        return
    
    client = genai.Client(api_key=api_key)
    
    config = types.GenerateContentConfig(
        tools=[types.Tool(code_execution=types.ToolCodeExecution())]
    )
    
    prompt = "Calculate the average of these corn yields: 150, 165, 142, 178, 156 bushels per acre. Use Python code to calculate mean and standard deviation."
    
    print(f"📤 Prompt: {prompt}")
    print("\n📥 Streaming response:")
    print("-" * 30)
    
    chunk_count = 0
    for chunk in client.models.generate_content_stream(
        model="gemini-2.5-flash",
        contents=prompt,
        config=config
    ):
        chunk_count += 1
        print(f"\n🔄 Chunk {chunk_count}:")
        print(f"   Type: {type(chunk)}")
        
        if hasattr(chunk, 'candidates') and chunk.candidates:
            for j, candidate in enumerate(chunk.candidates):
                print(f"   Candidate {j+1}:")
                
                if hasattr(candidate, 'content') and candidate.content:
                    if hasattr(candidate.content, 'parts') and candidate.content.parts:
                        for k, part in enumerate(candidate.content.parts):
                            print(f"     Part {k+1}: {type(part)}")
                            
                            if hasattr(part, 'text') and part.text:
                                print(f"       📝 Text: {part.text[:100]}{'...' if len(part.text) > 100 else ''}")
                                
                            if hasattr(part, 'executable_code') and part.executable_code:
                                print(f"       💻 EXECUTABLE CODE FOUND!")
                                print(f"         Language: {part.executable_code.language}")
                                print(f"         Code: {part.executable_code.code}")
                                
                            if hasattr(part, 'code_execution_result') and part.code_execution_result:
                                print(f"       ✅ CODE EXECUTION RESULT FOUND!")
                                print(f"         Outcome: {part.code_execution_result.outcome}")
                                print(f"         Output: {part.code_execution_result.output}")
                else:
                    print(f"     No content in candidate")
        else:
            print(f"   No candidates in chunk")
    
    print(f"\n📈 Total chunks received: {chunk_count}")

if __name__ == "__main__":
    try:
        test_code_execution_direct()
        test_streaming_code_execution()
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
