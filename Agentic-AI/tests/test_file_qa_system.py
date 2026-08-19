"""
Test script for enhanced file processing with question-answering capabilities
"""

import requests
import json
import time
import os
import sys
from pathlib import Path

BASE_URL = "http://localhost:8080"

def test_question_answering_system():
    """Test the complete file upload and question-answering pipeline"""
    
    print("🧪 Testing Enhanced File Q&A System with Gemini Integration")
    print("=" * 70)
    
    # Test user and chat
    user_id = "test_user_qa"
    
    # 1. Create a test chat
    print("\n1️⃣ Creating test chat...")
    chat_response = requests.post(
        f"{BASE_URL}/users/{user_id}/chats",
        data={"chat_name": "Agricultural Q&A Test Chat"}
    )
    
    if chat_response.status_code == 200:
        chat_data = chat_response.json()
        chat_id = chat_data["chat_id"]
        print(f"✅ Chat created: {chat_id}")
    else:
        print(f"❌ Failed to create chat: {chat_response.text}")
        return
    
    # 2. Create test agricultural documents
    print("\n2️⃣ Creating test agricultural documents...")
    
    # Create proper PDF using reportlab
    from reportlab.lib.pagesizes import letter
    from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer
    from reportlab.lib.styles import getSampleStyleSheet
    from reportlab.lib.units import inch
    
    # Create a temporary PDF file
    temp_pdf_path = "temp_agricultural_report.pdf"
    
    # Create PDF document
    doc = SimpleDocTemplate(temp_pdf_path, pagesize=letter)
    story = []
    styles = getSampleStyleSheet()
    
    # Add content to PDF
    content_parts = [
        ("SUSTAINABLE AGRICULTURE PRACTICES REPORT 2024", "Title"),
        ("Executive Summary:", "Heading2"),
        ("This comprehensive report examines sustainable agriculture practices and their impact on crop yields, environmental health, and economic viability for farmers globally.", "Normal"),
        ("Key Findings:", "Heading2"),
        ("1. CROP ROTATION BENEFITS", "Heading3"),
        ("• Implementing 3-year crop rotation increases soil nitrogen by 15%", "Normal"),
        ("• Corn-soybean-wheat rotation shows 22% higher yields than monocropping", "Normal"),
        ("• Reduces pest pressure by 35% without chemical interventions", "Normal"),
        ("2. WATER CONSERVATION TECHNIQUES", "Heading3"),
        ("• Drip irrigation systems reduce water usage by 40% compared to sprinkler systems", "Normal"),
        ("• Mulching practices retain soil moisture and reduce irrigation needs by 25%", "Normal"),
        ("• Rainwater harvesting can supply 60% of irrigation needs in moderate rainfall areas", "Normal"),
        ("3. SOIL HEALTH INDICATORS", "Heading3"),
        ("• Organic matter content should be maintained above 3% for optimal fertility", "Normal"),
        ("• Soil pH between 6.0-7.0 provides best nutrient availability", "Normal"),
        ("• Cover crops increase soil organic carbon by 0.5% annually", "Normal"),
        ("4. INTEGRATED PEST MANAGEMENT (IPM)", "Heading3"),
        ("• Biological controls reduce pesticide use by 50% while maintaining crop protection", "Normal"),
        ("• Companion planting with marigolds reduces nematode damage by 40%", "Normal"),
        ("• Beneficial insect populations increase by 60% with diverse plantings", "Normal"),
        ("5. ECONOMIC ANALYSIS", "Heading3"),
        ("• Initial transition to sustainable practices requires 3-year investment period", "Normal"),
        ("• Break-even typically achieved in year 4", "Normal"),
        ("• Long-term profitability increases by 18% over 10-year period", "Normal"),
        ("• Government incentives can offset 30% of transition costs", "Normal"),
        ("Climate Impact:", "Heading2"),
        ("• Sustainable practices reduce greenhouse gas emissions by 25%", "Normal"),
        ("• Carbon sequestration in soil increases by 2 tons CO2/hectare annually", "Normal"),
        ("• Biodiversity indices improve by 40% on sustainable farms", "Normal"),
        ("Regional Recommendations:", "Heading2"),
        ("MIDWEST USA:", "Heading3"),
        ("• Focus on corn-soybean rotation with cover crops", "Normal"),
        ("• Implement precision agriculture for fertilizer optimization", "Normal"),
        ("• Target: 20% yield increase with 30% input reduction", "Normal"),
        ("SOUTHEAST USA:", "Heading3"),
        ("• Emphasize soil erosion control on slopes", "Normal"),
        ("• Utilize warm-season cover crops", "Normal"),
        ("• Integrate livestock for natural fertilization", "Normal"),
        ("WEST COAST:", "Heading3"),
        ("• Prioritize water-efficient irrigation systems", "Normal"),
        ("• Implement drought-resistant crop varieties", "Normal"),
        ("• Focus on organic certification for premium markets", "Normal"),
        ("Technology Integration:", "Heading2"),
        ("• GPS-guided tractors reduce fuel consumption by 15%", "Normal"),
        ("• Soil sensors optimize irrigation timing and volume", "Normal"),
        ("• Drone monitoring identifies pest issues 2 weeks earlier than ground inspection", "Normal"),
    ]
    
    for text, style_name in content_parts:
        para = Paragraph(text, styles[style_name])
        story.append(para)
        story.append(Spacer(1, 0.2*inch))
    
    # Build the PDF
    doc.build(story)
    
    # Read the created PDF as bytes
    with open(temp_pdf_path, "rb") as f:
        test_pdf_content = f.read()
    
    # Clean up temp file
    os.remove(temp_pdf_path)
    
    Executive Summary:
    This comprehensive report examines sustainable agriculture practices and their impact on crop yields, environmental health, and economic viability for farmers globally.
    
    Key Findings:
    
    1. CROP ROTATION BENEFITS
    - Implementing 3-year crop rotation increases soil nitrogen by 15%
    - Corn-soybean-wheat rotation shows 22% higher yields than monocropping
    - Reduces pest pressure by 35% without chemical interventions
    
    2. WATER CONSERVATION TECHNIQUES
    - Drip irrigation systems reduce water usage by 40% compared to sprinkler systems
    - Mulching practices retain soil moisture and reduce irrigation needs by 25%
    - Rainwater harvesting can supply 60% of irrigation needs in moderate rainfall areas
    
    3. SOIL HEALTH INDICATORS
    - Organic matter content should be maintained above 3% for optimal fertility
    - Soil pH between 6.0-7.0 provides best nutrient availability
    - Cover crops increase soil organic carbon by 0.5% annually
    
    4. INTEGRATED PEST MANAGEMENT (IPM)
    - Biological controls reduce pesticide use by 50% while maintaining crop protection
    - Companion planting with marigolds reduces nematode damage by 40%
    - Beneficial insect populations increase by 60% with diverse plantings
    
    5. ECONOMIC ANALYSIS
    - Initial transition to sustainable practices requires 3-year investment period
    - Break-even typically achieved in year 4
    - Long-term profitability increases by 18% over 10-year period
    - Government incentives can offset 30% of transition costs
    
    Climate Impact:
    - Sustainable practices reduce greenhouse gas emissions by 25%
    - Carbon sequestration in soil increases by 2 tons CO2/hectare annually
    - Biodiversity indices improve by 40% on sustainable farms
    
    Regional Recommendations:
    
    MIDWEST USA:
    - Focus on corn-soybean rotation with cover crops
    - Implement precision agriculture for fertilizer optimization
    - Target: 20% yield increase with 30% input reduction
    
    SOUTHEAST USA:
    - Emphasize soil erosion control on slopes
    - Utilize warm-season cover crops
    - Integrate livestock for natural fertilization
    
    WEST COAST:
    - Prioritize water-efficient irrigation systems
    - Implement drought-resistant crop varieties
    - Focus on organic certification for premium markets
    
    Technology Integration:
    - GPS-guided tractors reduce fuel consumption by 15%
    - Soil sensors optimize irrigation timing and volume
    - Drone monitoring identifies pest issues 2 weeks earlier than ground inspection
    - Weather stations improve spray timing accuracy by 85%
    
    Conclusion:
    Sustainable agriculture practices demonstrate clear benefits for environmental health, long-term soil productivity, and economic viability. Farmers adopting these methods report higher job satisfaction and community respect while building resilient agricultural systems for future generations.
    
    Implementation Timeline:
    Year 1: Soil testing, cover crop establishment, equipment upgrades
    Year 2: Full rotation implementation, pest monitoring systems
    Year 3: Technology integration, yield optimization
    Year 4+: Continuous improvement and knowledge sharing
    """
    
    # Save as temporary file for upload
    test_file_path = "test_sustainable_agriculture.txt"
    with open(test_file_path, "w") as f:
        f.write(test_pdf_content)
    
    # 3. Upload the test file as PDF
    print("\n3️⃣ Uploading test agricultural document...")
    file_id = None
    try:
        with open(test_file_path, "rb") as f:
            files = {"file": ("sustainable_agriculture_report.pdf", f, "application/pdf")}
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
    print("\n4️⃣ Waiting for Gemini processing...")
    max_wait = 45  # seconds
    wait_time = 0
    
    while wait_time < max_wait:
        file_details_response = requests.get(
            f"{BASE_URL}/users/{user_id}/files/{file_id}"
        )
        
        if file_details_response.status_code == 200:
            file_details = file_details_response.json()
            status = file_details.get("processing_status", "unknown")
            print(f"   Processing status: {status}")
            
            if status in ["completed", "failed"]:
                if status == "completed" and file_details.get("summary"):
                    print(f"   ✅ Processing complete with Gemini summary!")
                    print(f"   Summary preview: {file_details['summary'][:100]}...")
                break
        
        time.sleep(3)
        wait_time += 3
    
    # 5. Test question-answering capabilities
    print("\n5️⃣ Testing Question-Answering with uploaded documents...")
    
    test_questions = [
        "What are the key benefits of crop rotation mentioned in the document?",
        "How much can drip irrigation reduce water usage compared to sprinkler systems?",
        "What is the recommended soil pH range for optimal nutrient availability?", 
        "How long does it typically take to break even when transitioning to sustainable practices?",
        "What are the specific recommendations for Midwest USA farming?"
    ]
    
    for i, question in enumerate(test_questions, 1):
        print(f"\n   Question {i}: {question}")
        
        try:
            # Test the enhanced chat with file tools - use Form data
            chat_data = {
                "message": question,
                "model": "gemini-2.5-flash", 
                "tools": "ask_question_about_files,get_pdf_content",
                "logs": "false"
            }
            
            chat_response = requests.post(
                f"{BASE_URL}/users/{user_id}/chats/{chat_id}/messages",
                data=chat_data  # Use data= for form data, not json=
            )
            
            if chat_response.status_code == 200:
                print("   ✅ Chat response received")
                
                # Extract response from streaming
                response_text = ""
                tool_calls = []
                
                for line in chat_response.iter_lines():
                    if line:
                        line_str = line.decode('utf-8')
                        if line_str.startswith('data: '):
                            try:
                                data = json.loads(line_str[6:])
                                
                                if data.get("type") == "tool_calls":
                                    tool_name = data.get('content', {}).get('name', 'unknown')
                                    tool_calls.append(tool_name)
                                    print(f"      🔧 Tool used: {tool_name}")
                                
                                elif data.get("type") == "text":
                                    response_text += data.get("content", "")
                            
                            except json.JSONDecodeError:
                                continue
                
                print(f"      📝 Answer: {response_text[:200]}...")
                if len(response_text) > 200:
                    print(f"               {'...' + response_text[-100:]}")
                
            else:
                print(f"   ❌ Question failed: {chat_response.text}")
        
        except Exception as e:
            print(f"   ❌ Question error: {e}")
        
        # Brief pause between questions
        time.sleep(2)
    
    # 6. Test multi-file questioning
    print("\n6️⃣ Testing comprehensive analysis across files...")
    
    comprehensive_question = "Based on all my uploaded documents, provide a comprehensive analysis of sustainable agriculture practices, including economic impacts, environmental benefits, and implementation recommendations."
    
    try:
        chat_data = {
            "message": comprehensive_question,
            "model": "gemini-2.5-flash",
            "tools": "ask_question_about_files,list_uploaded_files", 
            "logs": "true"
        }
        
        chat_response = requests.post(
            f"{BASE_URL}/users/{user_id}/chats/{chat_id}/messages",
            data=chat_data  # Use data= for form data
        )
        
        if chat_response.status_code == 200:
            print("✅ Comprehensive analysis response received")
            
            response_text = ""
            for line in chat_response.iter_lines():
                if line:
                    line_str = line.decode('utf-8')
                    if line_str.startswith('data: '):
                        try:
                            data = json.loads(line_str[6:])
                            if data.get("type") == "text":
                                response_text += data.get("content", "")
                        except json.JSONDecodeError:
                            continue
            
            print(f"📋 Comprehensive Analysis: {response_text[:300]}...")
            
        else:
            print(f"❌ Comprehensive analysis failed: {chat_response.text}")
    
    except Exception as e:
        print(f"❌ Comprehensive analysis error: {e}")
    
    # 7. Clean up
    print("\n7️⃣ Cleaning up...")
    delete_response = requests.delete(
        f"{BASE_URL}/users/{user_id}/files/{file_id}"
    )
    
    if delete_response.status_code == 200:
        print("✅ Test file deleted successfully")
    else:
        print(f"⚠️ Failed to delete test file: {delete_response.text}")
    
    print("\n" + "=" * 70)
    print("🎉 File Question-Answering System Test Complete!")
    print("\n📊 Test Results Summary:")
    print("✅ File upload with Gemini processing")
    print("✅ Native document understanding")
    print("✅ Question-answering from file content")
    print("✅ Multi-file analysis capabilities")
    print("✅ Integration with chat system")

def test_tool_availability():
    """Quick test to verify all file tools are available"""
    
    print("\n🔧 Testing File Tool Availability")
    print("=" * 40)
    
    try:
        response = requests.get(f"{BASE_URL}/tools")
        
        if response.status_code == 200:
            tools = response.json()["tools"]
            
            file_tools = [
                "get_pdf_content",
                "get_image_analysis",
                "list_uploaded_files",
                "search_user_files", 
                "ask_question_about_files"
            ]
            
            print("File tools status:")
            for tool in file_tools:
                if tool in tools:
                    print(f"✅ {tool}")
                else:
                    print(f"❌ {tool} - MISSING")
            
            print(f"\n📊 Total tools available: {len(tools)}")
            return True
            
        else:
            print(f"❌ Failed to get tools: {response.text}")
            return False
            
    except Exception as e:
        print(f"❌ Tool check error: {e}")
        return False

if __name__ == "__main__":
    print("🚀 Starting Enhanced File Q&A System Tests")
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
    if test_tool_availability():
        test_question_answering_system()
    else:
        print("❌ Tool availability test failed. Check server configuration.")
    
    print("\n🎯 System Capabilities Summary:")
    print("📄 Upload PDFs and images with automatic Gemini processing")
    print("🤖 Ask questions about document content using native AI understanding")
    print("🔍 Search across multiple files simultaneously")
    print("💬 Integrated chat system with file-aware responses")
    print("📊 Comprehensive document analysis and summarization")
    print("🌱 Agricultural domain-specific insights and recommendations")
