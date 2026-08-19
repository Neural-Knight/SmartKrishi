#!/usr/bin/env python3
"""
Debug test to check if the client manager streaming works
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

from app.client_manager import client_manager
from google.genai import types
import tempfile
import csv

def test_streaming_with_files():
    """Test if client manager streaming works with files"""
    
    # Create a test CSV in a simple way
    csv_file = "test_streaming_data.csv"
    
    # Write CSV content manually to ensure proper format
    with open(csv_file, 'w', newline='') as f:
        f.write('Product,Yield_Tons,Price_Per_Ton\n')
        f.write('Wheat,150.5,250\n')
        f.write('Corn,200.3,180\n')
        f.write('Barley,75.2,300\n')
    
    try:
        print("🧪 Testing Client Manager Streaming with Files")
        print("=" * 50)
        
        # Upload file to client manager
        chat_id = "test_streaming_debug"
        print(f"📤 Uploading CSV to client manager...")
        
        # Make sure file has proper CSV content and format
        print(f"📄 CSV file created: {csv_file}")
        with open(csv_file, 'r') as f:
            content = f.read()
            print(f"📄 File content: {content[:100]}...")
        
        uploaded_file = client_manager.upload_file(
            chat_id, 
            csv_file, 
            display_name="agricultural_data.csv",
            mime_type="text/csv"
        )
        if not uploaded_file:
            print("❌ Failed to upload file")
            return
        
        print(f"✅ File uploaded: {uploaded_file.uri}")
        
        # Create file part for streaming with explicit MIME type
        file_part = types.Part(file_data=types.FileData(
            file_uri=uploaded_file.uri,
            mime_type="text/csv"  # Explicitly specify CSV MIME type
        ))
        
        # Test streaming with file
        contents = [
            file_part,
            "I have uploaded a CSV file with agricultural data. Please analyze it using code execution and calculate the total yield."
        ]
        
        config = types.GenerateContentConfig(
            tools=[types.Tool(code_execution=types.ToolCodeExecution())]
        )
        
        print(f"🤖 Testing streaming generation...")
        
        response_chunks = []
        for chunk in client_manager.generate_content_stream(
            chat_id=chat_id,
            contents=contents,
            config=config,
            model="gemini-2.5-flash"
        ):
            candidates = getattr(chunk, 'candidates', None) or []
            if not candidates:
                continue
            candidate = candidates[0]
            parts = getattr(candidate.content, 'parts', None) or []
            
            for part in parts:
                # Handle text parts
                text = getattr(part, 'text', None)
                if text:
                    print(f"📝 Text chunk: {text[:100]}...")
                    response_chunks.append(text)
                
                # Handle executable code parts
                if hasattr(part, 'executable_code') and part.executable_code:
                    executable_code = part.executable_code
                    code = getattr(executable_code, 'code', '')
                    language = getattr(executable_code, 'language', 'python')
                    print(f"🔧 Code execution: {language}")
                    print(f"   Code: {code[:100]}...")
                
                # Handle code execution result parts
                if hasattr(part, 'code_execution_result') and part.code_execution_result:
                    result = part.code_execution_result
                    outcome = getattr(result, 'outcome', 'unknown')
                    output = getattr(result, 'output', '')
                    print(f"📊 Code result: {outcome}")
                    print(f"   Output: {output[:100]}...")
        
        full_response = ''.join(response_chunks)
        
        print("\n" + "=" * 50)
        print("🎯 STREAMING TEST RESULTS:")
        print(f"✅ File uploaded successfully: {uploaded_file.uri}")
        print(f"✅ Streaming worked: {len(response_chunks)} chunks received")
        print(f"✅ Total response length: {len(full_response)} characters")
        
        if "425.0" in full_response or "total" in full_response.lower():
            print("✅ Code execution likely worked (found calculation results)")
        else:
            print("⚠️  Code execution may not have worked (no calculation results found)")
        
        return True
        
    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    finally:
        # Cleanup
        try:
            os.unlink(csv_file)
            print(f"🧹 Cleaned up: {csv_file}")
        except:
            pass

if __name__ == "__main__":
    test_streaming_with_files()
