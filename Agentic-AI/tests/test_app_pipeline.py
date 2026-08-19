#!/usr/bin/env python3
"""
Test the complete app pipeline for file upload and main agent processing
"""

import os
import csv
import tempfile
import requests
import json
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

def create_test_csv():
    """Create a test CSV file"""
    temp_file = tempfile.NamedTemporaryFile(mode='w', suffix='.csv', delete=False, newline='')
    
    sample_data = [
        ['Farm_ID', 'Crop', 'Yield_Tons', 'Area_Acres', 'Year'],
        ['F001', 'Corn', 120.5, 50, 2023],
        ['F001', 'Corn', 135.2, 50, 2024],
        ['F002', 'Soybeans', 85.3, 40, 2023],
        ['F002', 'Soybeans', 92.1, 40, 2024]
    ]
    
    writer = csv.writer(temp_file)
    writer.writerows(sample_data)
    temp_file.close()
    
    return temp_file.name

def test_app_file_pipeline():
    """Test the complete app pipeline"""
    print("🧪 TESTING: Complete App File Upload Pipeline")
    print("=" * 60)
    
    base_url = "http://localhost:8080"
    user_id = "test_user_pipeline"
    chat_id = "test_chat_pipeline"
    
    # Step 1: Create test CSV
    csv_file = create_test_csv()
    print(f"📊 Created test CSV: {csv_file}")
    
    try:
        # Step 2: Upload CSV file to the app
        print("\n📤 Uploading CSV to app...")
        with open(csv_file, 'rb') as f:
            files = {'file': ('test_data.csv', f, 'text/csv')}
            data = {'user_id': user_id, 'chat_id': chat_id}
            
            response = requests.post(f"{base_url}/upload/csv", files=files, data=data)
            
        if response.status_code == 200:
            upload_result = response.json()
            print(f"✅ Upload successful: {upload_result}")
        else:
            print(f"❌ Upload failed: {response.status_code} - {response.text}")
            return
        
        # Step 3: Check if file is in the database
        print("\n📋 Checking uploaded files...")
        files_response = requests.get(f"{base_url}/chat/{chat_id}/files", params={"user_id": user_id})
        if files_response.status_code == 200:
            files_data = files_response.json()
            print(f"📁 Files in database: {len(files_data.get('files', []))}")
            for file_info in files_data.get('files', []):
                print(f"   - {file_info.get('original_filename')} (status: {file_info.get('processing_status')})")
                print(f"     Gemini file ID: {file_info.get('gemini_file_id')}")
                print(f"     Summary: {file_info.get('summary', 'No summary')[:100]}...")
        else:
            print(f"❌ Failed to get files: {files_response.status_code} - {files_response.text}")
        
        # Step 4: Ask the main agent to analyze the CSV
        print("\n🤖 Testing main agent with uploaded CSV...")
        
        query = "I uploaded a CSV file with agricultural data. Please analyze it using code execution and calculate the total yield."
        
        # Use streaming endpoint
        data = {
            'user_id': user_id,
            'chat_id': chat_id,
            'q': query,
            'logs': 'true'
        }
        
        response = requests.post(f"{base_url}/ask_stream", data=data, stream=True)
        
        if response.status_code == 200:
            print("📥 Streaming response:")
            print("-" * 40)
            
            for line in response.iter_lines():
                if line:
                    try:
                        event_data = json.loads(line.decode('utf-8'))
                        event_type = event_data.get('type')
                        
                        if event_type == 'plan':
                            print(f"🎯 Plan: {event_data.get('plan')}")
                        elif event_type == 'log':
                            print(f"📝 Log: {event_data.get('message')}")
                        elif event_type == 'tool_result':
                            print(f"🔧 Tool: {event_data.get('tool_name')} - {event_data.get('result', '')[:100]}...")
                        elif event_type == 'response':
                            print(f"💬 Response: {event_data.get('content', '')[:200]}...")
                        elif event_type == 'complete':
                            print("✅ Stream complete")
                            
                    except json.JSONDecodeError:
                        print(f"📄 Raw line: {line.decode('utf-8')[:100]}...")
        else:
            print(f"❌ Main agent request failed: {response.status_code} - {response.text}")
        
        print("\n" + "=" * 60)
        print("🎯 PIPELINE TEST COMPLETE")
        print("Check the output above to see if:")
        print("   ✅ File uploaded successfully")
        print("   ✅ File stored in database with Gemini file ID")
        print("   ✅ Main agent accessed the file for analysis")
        print("   ✅ Code execution worked with the uploaded CSV")
        print("=" * 60)
        
    except Exception as e:
        print(f"❌ Error during pipeline test: {e}")
        import traceback
        traceback.print_exc()
    
    finally:
        # Cleanup
        os.unlink(csv_file)
        print(f"\n🧹 Cleaned up: {csv_file}")

if __name__ == "__main__":
    test_app_file_pipeline()
