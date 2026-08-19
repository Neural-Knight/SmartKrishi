#!/usr/bin/env python3
"""
Debug test to check the actual response structure from streaming
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
from app.history import create_chat

def test_response_structure():
    """Test the actual response structure to see code execution parts"""
    
    print("🧪 DEBUGGING: Response Structure Analysis")
    print("=" * 50)
    
    # Create chat and upload file using proper pipeline
    user_id = "debug_user"
    chat_id = create_chat(user_id, "Debug test chat")
    
    # Create test CSV
    csv_file = "debug_test_data.csv"
    with open(csv_file, 'w', newline='') as f:
        f.write('Product,Yield_Tons\n')
        f.write('Wheat,150.5\n')
        f.write('Corn,200.3\n')
    
    try:
        # Upload using app pipeline
        from app.media import save_file
        with open(csv_file, 'rb') as f:
            file_content = f.read()
        
        upload_result = save_file(
            user=user_id,
            content=file_content,
            filename="debug_data.csv",
            kind="csv",
            chat_id=chat_id
        )
        
        print(f"📁 File uploaded: {upload_result.get('file_id', 'Failed')}")
        
        # Wait for processing
        import time
        time.sleep(2)
        
        # Test streaming response structure
        contents = ["Analyze this CSV file and calculate the total yield using code execution."]
        
        config = types.GenerateContentConfig(
            tools=[types.Tool(code_execution=types.ToolCodeExecution())]
        )
        
        print("\n🔍 Analyzing response structure...")
        
        chunk_count = 0
        for chunk in client_manager.generate_content_stream(
            chat_id=chat_id,
            contents=contents,
            config=config,
            model="gemini-2.5-flash"
        ):
            chunk_count += 1
            print(f"\n📦 CHUNK {chunk_count}:")
            print(f"   Type: {type(chunk)}")
            
            # Check if chunk has candidates
            candidates = getattr(chunk, 'candidates', None)
            if candidates:
                print(f"   Candidates: {len(candidates)}")
                
                for i, candidate in enumerate(candidates):
                    print(f"   Candidate {i}:")
                    
                    # Check content
                    content = getattr(candidate, 'content', None)
                    if content:
                        print(f"     Content type: {type(content)}")
                        
                        # Check parts
                        parts = getattr(content, 'parts', None)
                        if parts:
                            print(f"     Parts: {len(parts)}")
                            
                            for j, part in enumerate(parts):
                                print(f"       Part {j}: {type(part)}")
                                
                                # Check for different part types
                                if hasattr(part, 'text') and part.text:
                                    print(f"         TEXT: {part.text[:50]}...")
                                    
                                if hasattr(part, 'executable_code') and part.executable_code:
                                    code = getattr(part.executable_code, 'code', '')
                                    language = getattr(part.executable_code, 'language', 'unknown')
                                    print(f"         EXECUTABLE_CODE [{language}]: {code[:50]}...")
                                else:
                                    # Check if the attribute exists but is None/empty
                                    exec_code = getattr(part, 'executable_code', None)
                                    print(f"         executable_code: {exec_code}")
                                    
                                if hasattr(part, 'code_execution_result') and part.code_execution_result:
                                    result = part.code_execution_result
                                    outcome = getattr(result, 'outcome', 'unknown')
                                    output = getattr(result, 'output', '')
                                    print(f"         CODE_RESULT [{outcome}]: {output[:50]}...")
                                else:
                                    # Check if the attribute exists but is None/empty
                                    exec_result = getattr(part, 'code_execution_result', None)
                                    print(f"         code_execution_result: {exec_result}")
                                    
                                # Check for any other attributes
                                part_attrs = [attr for attr in dir(part) if not attr.startswith('_')]
                                print(f"         Attributes: {part_attrs}")
            else:
                print("   No candidates found")
                
            # Limit output to first few chunks for debugging
            if chunk_count >= 5:
                print("\n   ... (truncated after 5 chunks)")
                break
        
        print(f"\n✅ Total chunks processed: {chunk_count}")
        
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
    
    finally:
        # Cleanup
        try:
            os.unlink(csv_file)
        except:
            pass

if __name__ == "__main__":
    test_response_structure()
