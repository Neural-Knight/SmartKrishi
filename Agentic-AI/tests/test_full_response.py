"""
Debug tool to analyze FULL response structure including code execution parts
"""
import asyncio
import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.client_manager import client_manager

async def test_full_response_access():
    print("🧪 DEBUGGING: Full Response Analysis")
    print("=" * 50)
    
    # Upload test CSV file that we know triggers code execution
    file_path = r"a:\Documents\CapitalOne\agent\uploads\demo_user\pokemon_20250817_111704_6bb204.csv"
    with open(file_path, "rb") as f:
        file_id = await client_manager.upload_file(f, "pokemon_data.csv", "text/csv")
    print(f"📁 File uploaded: {file_id}")
    
    print("\n🔍 Analyzing FULL response structure...")
    
    # Test with a request that should trigger code execution  
    messages = [
        {"role": "user", "content": f"Analyze this agricultural data CSV file and calculate the total yield. Execute Python code to process the data: {file_id}"}
    ]
    
    chunk_count = 0
    async for chunk in client_manager.generate_content_stream(messages):
        chunk_count += 1
        print(f"\n📦 CHUNK {chunk_count}:")
        print(f"   Type: {type(chunk)}")
        print(f"   Candidates: {len(chunk.candidates)}")
        
        for candidate_idx, candidate in enumerate(chunk.candidates):
            print(f"   Candidate {candidate_idx}:")
            print(f"     Content type: {type(candidate.content)}")
            print(f"     Parts: {len(candidate.content.parts)}")
            
            for part_idx, part in enumerate(candidate.content.parts):
                print(f"       Part {part_idx}: {type(part)}")
                
                # Check if this is a text part
                if hasattr(part, 'text') and part.text:
                    print(f"         TEXT: {part.text[:50]}...")
                
                # Check if this is an executable code part
                if hasattr(part, 'executable_code') and part.executable_code:
                    print(f"         🔥 EXECUTABLE CODE:")
                    print(f"         Language: {getattr(part.executable_code, 'language', 'unknown')}")
                    print(f"         Code: {getattr(part.executable_code, 'code', 'N/A')[:100]}...")
                
                # Check if this is a code execution result part
                if hasattr(part, 'code_execution_result') and part.code_execution_result:
                    print(f"         ✅ CODE EXECUTION RESULT:")
                    print(f"         Outcome: {getattr(part.code_execution_result, 'outcome', 'unknown')}")
                    output = getattr(part.code_execution_result, 'output', '')
                    print(f"         Output: {output[:100]}...")
                
                # Check all available attributes
                part_attrs = [attr for attr in dir(part) if not attr.startswith('_')]
                non_none_attrs = []
                for attr in part_attrs:
                    try:
                        value = getattr(part, attr)
                        if value is not None and not callable(value):
                            non_none_attrs.append(f"{attr}: {type(value)}")
                    except:
                        pass
                
                if non_none_attrs:
                    print(f"         Non-None Attributes: {', '.join(non_none_attrs)}")
        
        # Break after processing a few chunks to avoid overwhelming output
        if chunk_count >= 5:
            break
    
    print(f"\n✅ Total chunks processed: {chunk_count}")

if __name__ == "__main__":
    asyncio.run(test_full_response_access())
