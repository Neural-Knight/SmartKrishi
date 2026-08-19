"""
Simple test to verify file tools registration
"""
import requests
import json

def test_tools_registration():
    """Test if file tools are properly registered"""
    try:
        response = requests.get("http://localhost:8080/tools", timeout=5)
        if response.status_code == 200:
            tools = response.json()["tools"]
            print("✅ Server is running")
            print(f"📋 Available tools: {len(tools)} total")
            
            file_tools = [
                "get_pdf_content",
                "get_image_analysis",
                "list_uploaded_files", 
                "search_user_files"
            ]
            
            for tool in file_tools:
                if tool in tools:
                    print(f"✅ {tool}")
                else:
                    print(f"❌ {tool} - MISSING")
            
            return True
        else:
            print(f"❌ Server error: {response.status_code}")
            return False
            
    except Exception as e:
        print(f"❌ Connection failed: {e}")
        return False

if __name__ == "__main__":
    if test_tools_registration():
        print("\n🎉 File tools registration test passed!")
    else:
        print("\n💥 File tools registration test failed!")
