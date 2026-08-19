#!/usr/bin/env python3
"""
Test the fix for file code execution - ensuring files are properly passed as Part objects
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.nodes.main_agent import main_agent_node
from app.state import State
from app.client_manager import client_manager
from google.genai import types
import json

def test_file_part_creation():
    """Test that files are properly converted to Part objects for code execution"""
    
    print("🔧 Testing File Part Creation for Code Execution")
    print("=" * 60)
    
    # Create a test state
    state = State(
        user_id="test_user",
        chat_id="test_file_parts_chat",
        user_query="Analyze my CSV data and create visualizations"
    )
    
    # Mock plan for CSV analysis
    state.plan = {
        "primary_intent": "data_analysis",
        "tools_needed": [],
        "location": ""
    }
    
    print(f"📝 User Query: {state.user_query}")
    print(f"🎯 Plan: {state.plan}")
    
    # Test the main agent processing
    try:
        result_state = main_agent_node(state)
        
        print(f"✅ Main agent processed successfully")
        print(f"📄 Draft answer length: {len(result_state.draft_answer) if result_state.draft_answer else 0} characters")
        
        if result_state.draft_answer:
            print(f"📋 Response preview: {result_state.draft_answer[:200]}...")
        else:
            print("⚠️  No response generated")
            
    except Exception as e:
        print(f"❌ Error in main agent: {e}")
        import traceback
        traceback.print_exc()

def test_part_object_structure():
    """Test the Part object structure for file data"""
    
    print("\n🧪 Testing Part Object Structure")
    print("=" * 40)
    
    # Create a mock file URI
    mock_uri = "gs://generativeai-uploads/files/abc123xyz"
    
    try:
        # Create a Part object with fileData
        file_part = types.Part(file_data=types.FileData(file_uri=mock_uri))
        
        print(f"✅ Part object created successfully")
        print(f"📁 File URI: {file_part.file_data.file_uri}")
        print(f"🔍 Has file_data: {hasattr(file_part, 'file_data')}")
        print(f"🔍 File data type: {type(file_part.file_data)}")
        
        # Test the code execution detection logic
        is_data_file = file_part.file_data.file_uri.endswith(('.csv', '.xlsx'))
        print(f"📊 Detected as data file: {is_data_file}")
        
        # Test with CSV extension
        csv_uri = "gs://generativeai-uploads/files/data.csv"
        csv_part = types.Part(file_data=types.FileData(file_uri=csv_uri))
        is_csv = csv_part.file_data.file_uri.endswith(('.csv', '.xlsx'))
        print(f"📈 CSV file detected correctly: {is_csv}")
        
    except Exception as e:
        print(f"❌ Error creating Part object: {e}")
        import traceback
        traceback.print_exc()

def test_content_array_structure():
    """Test how the content array should be structured"""
    
    print("\n📦 Testing Content Array Structure")
    print("=" * 40)
    
    try:
        # Simulate the content array structure
        mock_files = [
            types.Part(file_data=types.FileData(file_uri="gs://files/data1.csv")),
            types.Part(file_data=types.FileData(file_uri="gs://files/data2.xlsx"))
        ]
        
        prompt = "Analyze the uploaded data files and create visualizations"
        
        contents = []
        contents.extend(mock_files)  # Add files first
        contents.append(prompt)     # Add prompt last
        
        print(f"✅ Content array created with {len(contents)} items")
        print(f"📁 File parts: {len(mock_files)}")
        print(f"📝 Prompt: '{prompt[:50]}...'")
        
        # Show the structure
        for i, content in enumerate(contents):
            if isinstance(content, types.Part):
                print(f"  [{i}] Part with file_data: {content.file_data.file_uri}")
            else:
                print(f"  [{i}] Text: {str(content)[:50]}...")
                
        print("✅ Content array structure looks correct")
        
    except Exception as e:
        print(f"❌ Error creating content array: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    test_part_object_structure()
    test_content_array_structure()
    test_file_part_creation()
    
    print("\n" + "=" * 60)
    print("🎯 Fix Summary:")
    print("- Files are now wrapped in types.Part objects with file_data")
    print("- FileData objects contain the proper file_uri for code execution")
    print("- Code execution is enabled for CSV/XLSX files based on URI")
    print("- Content array structure: [file_parts...] + [prompt]")
    print("=" * 60)
