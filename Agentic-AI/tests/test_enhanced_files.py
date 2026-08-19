"""
Comprehensive test script for the enhanced file processing system
"""

import requests
import json
import time
import os
from pathlib import Path

BASE_URL = "http://localhost:8080"

def test_file_processing_pipeline():
    """Test the complete file processing pipeline with Gemini integration"""
    
    print("🧪 Testing Enhanced File Processing Pipeline")
    print("=" * 60)
    
    # Test user and chat
    user_id = "test_user_files"
    
    # 1. Create a test chat
    print("\n1️⃣ Creating test chat...")
    chat_response = requests.post(
        f"{BASE_URL}/users/{user_id}/chats",
        data={"chat_name": "File Processing Test Chat"}
    )
    
    if chat_response.status_code == 200:
        chat_data = chat_response.json()
        chat_id = chat_data["chat_id"]
        print(f"✅ Chat created: {chat_id}")
    else:
        print(f"❌ Failed to create chat: {chat_response.text}")
        return
    
    # 2. Create a test PDF file
    print("\n2️⃣ Creating test PDF content...")
    test_pdf_content = """
    Agricultural Sustainability Report 2024
    
    Executive Summary:
    This report outlines key strategies for sustainable agriculture practices.
    
    Key Findings:
    - Soil health improvement techniques show 25% yield increase
    - Water conservation methods reduce usage by 30%
    - Integrated pest management reduces chemical use by 40%
    
    Recommendations:
    1. Implement crop rotation systems
    2. Adopt precision irrigation
    3. Use biological pest control methods
    
    Conclusion:
    Sustainable agriculture practices are essential for long-term food security.
    """
    
    # Create a simple PDF (for testing, we'll simulate with a text file)
    test_file_path = "test_agricultural_report.txt"
    with open(test_file_path, "w") as f:
        f.write(test_pdf_content)
    
    # 3. Upload the test file as PDF
    print("\n3️⃣ Uploading test file...")
    try:
        with open(test_file_path, "rb") as f:
            files = {"file": ("agricultural_report.pdf", f, "application/pdf")}
            data = {"user_id": user_id, "chat_id": chat_id}
            
            upload_response = requests.post(
                f"{BASE_URL}/upload/pdf",
                files=files,
                data=data
            )
        
        if upload_response.status_code == 200:
            upload_data = upload_response.json()
            file_id = upload_data["file_id"]
            print(f"✅ File uploaded successfully!")
            print(f"   File ID: {file_id}")
            print(f"   Processing Status: {upload_data['processing_status']}")
        else:
            print(f"❌ File upload failed: {upload_response.text}")
            return
            
    except Exception as e:
        print(f"❌ Upload error: {e}")
        return
    finally:
        # Clean up test file
        if os.path.exists(test_file_path):
            os.remove(test_file_path)
    
    # 4. Wait for processing to complete
    print("\n4️⃣ Waiting for file processing...")
    max_wait = 30  # seconds
    wait_time = 0
    file_details_response = None
    
    while wait_time < max_wait:
        file_details_response = requests.get(
            f"{BASE_URL}/users/{user_id}/files/{file_id}"
        )
        
        if file_details_response.status_code == 200:
            file_details = file_details_response.json()
            status = file_details.get("processing_status", "unknown")
            print(f"   Processing status: {status}")
            
            if status in ["completed", "failed"]:
                break
        
        time.sleep(2)
        wait_time += 2
    
    # 5. Check file details
    print("\n5️⃣ Checking file details...")
    if file_details_response and file_details_response.status_code == 200:
        file_details = file_details_response.json()
        print(f"✅ File Details Retrieved:")
        print(f"   Filename: {file_details['original_filename']}")
        print(f"   Type: {file_details['file_type']}")
        print(f"   Size: {file_details['file_size']} bytes")
        print(f"   Status: {file_details['processing_status']}")
        if file_details.get("summary"):
            print(f"   Summary: {file_details['summary'][:100]}...")
        if file_details.get("gemini_file_id"):
            print(f"   Gemini File ID: {file_details['gemini_file_id']}")
    else:
        error_msg = file_details_response.text if file_details_response else "No response received"
        print(f"❌ Failed to get file details: {error_msg}")
    
    # 6. List files in chat
    print("\n6️⃣ Listing files in chat...")
    chat_files_response = requests.get(
        f"{BASE_URL}/users/{user_id}/chats/{chat_id}/files"
    )
    
    if chat_files_response.status_code == 200:
        chat_files = chat_files_response.json()
        print(f"✅ Found {chat_files['total_count']} files in chat:")
        for file in chat_files["files"]:
            print(f"   - {file['original_filename']} ({file['file_type']}) - {file['processing_status']}")
    else:
        print(f"❌ Failed to list chat files: {chat_files_response.text}")
    
    # 7. Search files
    print("\n7️⃣ Searching files...")
    search_response = requests.get(
        f"{BASE_URL}/users/{user_id}/files/search",
        params={"query": "agriculture", "chat_id": chat_id}
    )
    
    if search_response.status_code == 200:
        search_results = search_response.json()
        print(f"✅ Search found {search_results['total_matches']} matches for 'agriculture':")
        for file in search_results["files"]:
            print(f"   - {file['original_filename']}")
    else:
        print(f"❌ Search failed: {search_response.text}")
    
    # 8. Test chat with file references
    print("\n8️⃣ Testing chat with file tools...")
    
    # Test message that should trigger file tools
    test_message = f"What are the key findings in the uploaded agricultural report? Please get the PDF content for file ID {file_id}."
    
    try:
        chat_response = requests.post(
            f"{BASE_URL}/users/{user_id}/chats/{chat_id}/messages",
            json={
                "message": test_message,
                "model": "gemini-2.0-flash-exp",
                "tools": ["get_pdf_content", "list_uploaded_files"],
                "logs": True
            }
        )
        
        if chat_response.status_code == 200:
            print("✅ Chat response received - checking for file tool usage...")
            
            # Stream the response to see tool calls
            response_text = ""
            for line in chat_response.iter_lines():
                if line:
                    line_str = line.decode('utf-8')
                    if line_str.startswith('data: '):
                        try:
                            data = json.loads(line_str[6:])
                            
                            if data.get("type") == "tool_calls":
                                print(f"   🔧 Tool called: {data.get('content', {}).get('name', 'unknown')}")
                            
                            elif data.get("type") == "tool_result":
                                tool_result = data.get("content", {})
                                print(f"   📋 Tool result: {str(tool_result)[:100]}...")
                            
                            elif data.get("type") == "text":
                                response_text += data.get("content", "")
                        
                        except json.JSONDecodeError:
                            continue
            
            print(f"\n📝 Final Response Preview: {response_text[:200]}...")
            
        else:
            print(f"❌ Chat failed: {chat_response.text}")
    
    except Exception as e:
        print(f"❌ Chat error: {e}")
    
    # 9. Clean up - delete the file
    print("\n9️⃣ Cleaning up test file...")
    delete_response = requests.delete(
        f"{BASE_URL}/users/{user_id}/files/{file_id}"
    )
    
    if delete_response.status_code == 200:
        print("✅ Test file deleted successfully")
    else:
        print(f"⚠️ Failed to delete test file: {delete_response.text}")
    
    print("\n" + "=" * 60)
    print("🎉 File Processing Pipeline Test Complete!")

def test_file_tools_directly():
    """Test the file tools directly by examining the tools endpoint"""
    
    print("\n🔧 Testing File Tools Registration")
    print("=" * 40)
    
    # Check if file tools are registered
    tools_response = requests.get(f"{BASE_URL}/tools")
    
    if tools_response.status_code == 200:
        tools = tools_response.json()["tools"]
        
        file_tools = [
            "get_pdf_content",
            "get_image_analysis", 
            "list_uploaded_files",
            "search_user_files"
        ]
        
        print("Available tools:")
        for tool in tools:
            status = "✅" if tool in file_tools else "🔍"
            print(f"  {status} {tool}")
        
        missing_tools = [tool for tool in file_tools if tool not in tools]
        if missing_tools:
            print(f"\n❌ Missing file tools: {missing_tools}")
        else:
            print(f"\n✅ All file tools registered successfully!")
            
    else:
        print(f"❌ Failed to get tools list: {tools_response.text}")

if __name__ == "__main__":
    print("🚀 Starting Enhanced File Processing Tests")
    print("Make sure the server is running on http://localhost:8080")
    
    # Test if server is running
    try:
        health_response = requests.get(f"{BASE_URL}/tools", timeout=5)
        if health_response.status_code == 200:
            print("✅ Server is running")
        else:
            print("❌ Server responding but with errors")
            exit(1)
    except requests.exceptions.RequestException:
        print("❌ Server is not running. Please start it first.")
        print("Run: python -m uvicorn app.main:api --reload --port 8080")
        exit(1)
    
    # Run tests
    test_file_tools_directly()
    test_file_processing_pipeline()
    
    print("\n🎯 Test Summary:")
    print("- File upload and processing with Gemini integration")
    print("- Automatic summary generation and metadata storage") 
    print("- File management endpoints (list, search, delete)")
    print("- File tools for Gemini to access uploaded content")
    print("- Chat integration with file references")
