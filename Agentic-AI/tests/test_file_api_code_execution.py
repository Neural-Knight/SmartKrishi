#!/usr/bin/env python3
"""
Comprehensive test of Gemini Files API + Code Execution
Tests if uploaded CSV files are accessible in the code execution environment
"""

import os
import csv
import tempfile
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

from google import genai
from google.genai import types

def create_sample_csv():
    """Create a sample agricultural CSV file for testing"""
    
    # Create a temporary CSV file
    temp_file = tempfile.NamedTemporaryFile(mode='w', suffix='.csv', delete=False, newline='')
    
    sample_data = [
        ['Farm_ID', 'Crop', 'Yield_Tons', 'Area_Acres', 'Year'],
        ['F001', 'Corn', 120.5, 50, 2023],
        ['F001', 'Corn', 135.2, 50, 2024],
        ['F002', 'Soybeans', 85.3, 40, 2023],
        ['F002', 'Soybeans', 92.1, 40, 2024],
        ['F003', 'Wheat', 67.8, 30, 2023],
        ['F003', 'Wheat', 71.4, 30, 2024],
        ['F004', 'Corn', 142.7, 60, 2023],
        ['F004', 'Corn', 156.3, 60, 2024],
        ['F005', 'Soybeans', 78.9, 35, 2023],
        ['F005', 'Soybeans', 84.2, 35, 2024]
    ]
    
    writer = csv.writer(temp_file)
    writer.writerows(sample_data)
    temp_file.close()
    
    print(f"📊 Created sample CSV file: {temp_file.name}")
    print("📋 Sample data:")
    for i, row in enumerate(sample_data[:4]):  # Show first few rows
        print(f"   {row}")
    if len(sample_data) > 4:
        print(f"   ... and {len(sample_data) - 4} more rows")
    
    return temp_file.name

def test_file_upload_and_code_execution():
    """Test uploading a CSV file and accessing it in code execution"""
    
    print("\n🧪 TESTING: File Upload + Code Execution Integration")
    print("=" * 60)
    
    # Check for API key
    api_key = os.getenv('GOOGLE_API_KEY') or os.getenv('GEMINI_API_KEY')
    if not api_key:
        print("❌ No GOOGLE_API_KEY found - cannot test file operations")
        return
    
    try:
        # Create client
        client = genai.Client(api_key=api_key)
        print("✅ Gemini client created successfully")
        
        # Create sample CSV
        csv_file_path = create_sample_csv()
        
        # Upload the file
        print("\n📤 Uploading CSV file to Gemini Files API...")
        uploaded_file = client.files.upload(
            file=csv_file_path,
            config={
                "display_name": "agricultural_data.csv",
                "mime_type": "text/csv"
            }
        )
        
        print(f"✅ File uploaded successfully!")
        print(f"   📁 File name: {uploaded_file.name}")
        print(f"   📄 Display name: {uploaded_file.display_name}")
        print(f"   🔗 URI: {uploaded_file.uri}")
        print(f"   📊 Size: {uploaded_file.size_bytes} bytes")
        
        # Test 1: Use the file in a chat with code execution
        print("\n🧠 TEST 1: Chat with file and code execution")
        print("-" * 40)
        
        # Create proper Part object with fileData
        file_part = types.Part(file_data=types.FileData(file_uri=uploaded_file.uri))
        
        prompt = """I have uploaded an agricultural CSV file with farm yield data. 
Please analyze this data and:

1. Load the CSV data
2. Calculate the total yield across all farms
3. Find the average yield per acre
4. Show which crop has the highest total yield
5. Create a summary of the analysis

Please use Python code execution to perform these calculations."""
        
        # Configure chat with code execution
        chat = client.chats.create(
            model="gemini-2.5-flash",
            config=types.GenerateContentConfig(
                tools=[types.Tool(code_execution=types.ToolCodeExecution())]
            )
        )
        
        print("📝 Sending prompt with uploaded file...")
        response = chat.send_message([file_part, prompt])
        
        print("\n📋 RESPONSE ANALYSIS:")
        print("=" * 30)
        
        if response.candidates:
            for i, part in enumerate(response.candidates[0].content.parts):
                print(f"\n[Part {i+1}]")
                
                if part.text:
                    print(f"💬 Text: {part.text[:200]}...")
                
                if part.executable_code:
                    print(f"💻 Code executed:")
                    print(f"```python\n{part.executable_code.code}\n```")
                
                if part.code_execution_result:
                    print(f"📊 Code output:")
                    print(f"```\n{part.code_execution_result.output}\n```")
        
        # Test 2: Direct content generation with file
        print("\n🧠 TEST 2: Direct generate_content with file")
        print("-" * 40)
        
        direct_response = client.models.generate_content(
            model="gemini-2.5-flash",
            contents=[file_part, "Calculate the total yield from this CSV file using Python code."],
            config=types.GenerateContentConfig(
                tools=[types.Tool(code_execution=types.ToolCodeExecution())]
            )
        )
        
        print("📋 Direct response:")
        if direct_response.candidates:
            for part in direct_response.candidates[0].content.parts:
                if part.text:
                    print(f"💬 {part.text[:200]}...")
                if part.executable_code:
                    print(f"💻 Executed: {part.executable_code.code[:100]}...")
                if part.code_execution_result:
                    print(f"📊 Result: {part.code_execution_result.output[:200]}...")
        
        # Test 3: Check if file is accessible in code environment
        print("\n🧠 TEST 3: File accessibility check")
        print("-" * 40)
        
        accessibility_response = client.models.generate_content(
            model="gemini-2.5-flash",
            contents=[
                file_part,
                """Use Python code to:
1. Check if the uploaded file is accessible
2. List all available files in the environment
3. Try to read the CSV file and show the first few rows

Please execute Python code to do this analysis."""
            ],
            config=types.GenerateContentConfig(
                tools=[types.Tool(code_execution=types.ToolCodeExecution())]
            )
        )
        
        print("📋 Accessibility check:")
        if accessibility_response.candidates:
            for part in accessibility_response.candidates[0].content.parts:
                if part.executable_code:
                    print(f"💻 Code: {part.executable_code.code}")
                if part.code_execution_result:
                    print(f"📊 Output: {part.code_execution_result.output}")
        
        # Cleanup
        os.unlink(csv_file_path)
        print(f"\n🧹 Cleaned up temporary file: {csv_file_path}")
        
        print("\n" + "=" * 60)
        print("🎯 TEST COMPLETE - Check the results above to see if:")
        print("   ✅ File was uploaded successfully")
        print("   ✅ Code execution worked with the file")
        print("   ✅ File data was accessible in Python environment")
        print("=" * 60)
        
    except Exception as e:
        print(f"❌ Error during testing: {e}")
        import traceback
        traceback.print_exc()

def test_simple_code_execution():
    """Test basic code execution without files"""
    
    print("\n🧪 TESTING: Basic Code Execution (No Files)")
    print("=" * 50)
    
    api_key = os.getenv('GOOGLE_API_KEY') or os.getenv('GEMINI_API_KEY')
    if not api_key:
        print("❌ No GOOGLE_API_KEY found")
        return
    
    try:
        client = genai.Client(api_key=api_key)
        
        # Test basic math calculation
        chat = client.chats.create(
            model="gemini-2.5-flash",
            config=types.GenerateContentConfig(
                tools=[types.Tool(code_execution=types.ToolCodeExecution())]
            )
        )
        
        response = chat.send_message(
            "Calculate the sum of the first 10 prime numbers using Python code. "
            "Show me the code and the result."
        )
        
        print("📋 Basic code execution test:")
        for part in response.candidates[0].content.parts:
            if part.text:
                print(f"💬 Text: {part.text}")
            if part.executable_code:
                print(f"💻 Code: {part.executable_code.code}")
            if part.code_execution_result:
                print(f"📊 Result: {part.code_execution_result.output}")
                
    except Exception as e:
        print(f"❌ Error: {e}")

if __name__ == "__main__":
    print("🚀 COMPREHENSIVE FILE API + CODE EXECUTION TEST")
    print("=" * 60)
    
    # Test basic code execution first
    test_simple_code_execution()
    
    # Test file upload and code execution
    test_file_upload_and_code_execution()
