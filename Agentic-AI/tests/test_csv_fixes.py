"""
Test script to verify CSV file upload and analysis fixes.
"""

import os
import sys
import tempfile
import csv
import logging

# Add the project root to the Python path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.client_manager import client_manager
from app.media import save_file
from app.main import api

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("CSVTest")

def create_test_csv():
    """Create a test CSV file with sample agricultural data"""
    test_data = [
        ["Date", "Crop", "Yield_tons", "Temperature_C", "Rainfall_mm", "Soil_pH"],
        ["2024-01-01", "Wheat", "4.2", "15.5", "45.2", "6.8"],
        ["2024-01-02", "Wheat", "4.1", "16.2", "42.1", "6.7"],
        ["2024-01-03", "Corn", "8.5", "22.1", "38.5", "6.9"],
        ["2024-01-04", "Corn", "8.8", "23.5", "35.2", "7.0"],
        ["2024-01-05", "Soybeans", "3.2", "20.8", "55.3", "6.5"],
    ]
    
    # Create temporary CSV file
    temp_file = tempfile.NamedTemporaryFile(mode='w', suffix='.csv', delete=False)
    writer = csv.writer(temp_file)
    writer.writerows(test_data)
    temp_file.close()
    
    return temp_file.name

def test_csv_upload_and_analysis():
    """Test CSV upload and analysis with the new client manager"""
    
    print("🧪 Testing CSV Upload and Analysis...")
    
    # Create test CSV
    csv_path = create_test_csv()
    print(f"📄 Created test CSV: {csv_path}")
    
    try:
        # Test parameters
        user_id = "test_user"
        chat_id = "test_chat_123"
        filename = "test_agricultural_data.csv"
        
        # Read file content
        with open(csv_path, 'rb') as f:
            file_content = f.read()
        
        print(f"📤 Uploading CSV file...")
        
        # Test file upload
        result = save_file(user_id, file_content, filename, "csv", chat_id)
        
        if result.get("success"):
            print(f"✅ File uploaded successfully!")
            print(f"   File ID: {result['file_id']}")
            print(f"   Processing Status: {result['processing_status']}")
            if result.get('summary'):
                print(f"   Summary: {result['summary'][:200]}...")
        else:
            print(f"❌ File upload failed: {result.get('error')}")
            return False
        
        # Test client manager file access
        print(f"🔍 Testing file access via client manager...")
        
        chat_files = client_manager.list_chat_files(chat_id)
        print(f"📁 Found {len(chat_files)} files for chat {chat_id}")
        
        for file_ref in chat_files:
            if hasattr(file_ref, 'display_name'):
                print(f"   - {file_ref.display_name}")
            elif hasattr(file_ref, 'name'):
                print(f"   - {file_ref.name}")
        
        # Test code execution with the file
        print(f"🔬 Testing code execution with uploaded file...")
        
        if chat_files:
            test_prompt = """
            Analyze the uploaded CSV file. Please:
            1. Load the data and show its structure
            2. Calculate basic statistics
            3. Identify any patterns in the agricultural data
            
            Use pandas to analyze the data properly.
            """
            
            contents = chat_files + [test_prompt]
            
            from google.genai import types
            
            response = client_manager.generate_content(
                chat_id=chat_id,
                contents=contents,
                config=types.GenerateContentConfig(
                    tools=[types.Tool(code_execution=types.ToolCodeExecution())]
                )
            )
            
            if response and hasattr(response, 'text') and response.text:
                print(f"✅ Code execution successful!")
                print(f"   Response: {response.text[:300]}...")
            else:
                print(f"⚠️  Code execution completed but no text response")
        
        print(f"🧹 Cleanup...")
        
    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    finally:
        # Cleanup
        if os.path.exists(csv_path):
            os.unlink(csv_path)
    
    print(f"✅ CSV test completed successfully!")
    return True

def test_client_persistence():
    """Test that client manager maintains separate clients per chat"""
    
    print("🔄 Testing client persistence and isolation...")
    
    chat1 = "chat_001"
    chat2 = "chat_002"
    
    client1 = client_manager.get_client(chat1)
    client2 = client_manager.get_client(chat2)
    
    # Should be different client instances
    if client1 is not client2:
        print("✅ Clients are properly isolated per chat")
    else:
        print("❌ Clients are not isolated - same instance returned")
        return False
    
    # Should return same client for same chat
    client1_again = client_manager.get_client(chat1)
    if client1 is client1_again:
        print("✅ Client persistence works - same instance returned for same chat")
    else:
        print("❌ Client persistence failed - different instances for same chat")
        return False
    
    return True

if __name__ == "__main__":
    print("🚀 Starting CSV File Analysis Tests...\n")
    
    # Check if API key is available
    if not os.getenv('GOOGLE_API_KEY') and not os.getenv('GEMINI_API_KEY'):
        print("❌ No GOOGLE_API_KEY or GEMINI_API_KEY found in environment")
        print("   Please set one of these environment variables to run the test")
        sys.exit(1)
    
    success = True
    
    # Test client persistence
    if not test_client_persistence():
        success = False
    
    print()
    
    # Test CSV upload and analysis
    if not test_csv_upload_and_analysis():
        success = False
    
    print(f"\n{'='*50}")
    if success:
        print("🎉 All tests passed! CSV file analysis is working correctly.")
    else:
        print("💥 Some tests failed. Please check the implementation.")
    print(f"{'='*50}")
