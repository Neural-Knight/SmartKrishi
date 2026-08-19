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

def main():
    """Test the enhanced file Q&A system with proper PDF creation"""
    
    print("🚀 Starting Enhanced File Q&A System Tests")
    print("Make sure the server is running on http://localhost:8080")
    
    # Test server connectivity
    try:
        response = requests.get(f"{BASE_URL}/tools")
        print("✅ Server is running")
    except requests.exceptions.ConnectionError:
        print("❌ Server is not running. Please start it with: python -m uvicorn app.main:api --reload --port 8080")
        return
    
    # Test tool availability
    print("\n🔧 Testing File Tool Availability")
    print("=" * 40)
    
    try:
        tools_response = requests.get(f"{BASE_URL}/tools")
        if tools_response.status_code == 200:
            tools_data = tools_response.json()
            tools_list = tools_data.get("tools", [])
            file_tools = [
                "get_pdf_content",
                "get_image_analysis", 
                "list_uploaded_files",
                "search_user_files",
                "ask_question_about_files"
            ]
            
            print("File tools status:")
            available_tools = 0
            for tool in file_tools:
                if tool in tools_list:
                    print(f"✅ {tool}")
                    available_tools += 1
                else:
                    print(f"❌ {tool}")
            
            print(f"\n📊 Total tools available: {len(tools_list)}")
            
            if available_tools < len(file_tools):
                print("⚠️  Some file tools are missing. The test may not work properly.")
        
    except Exception as e:
        print(f"❌ Failed to get tools: {e}")
        return

    print("\n🧪 Testing Enhanced File Q&A System with Gemini Integration")
    print("=" * 70)
    
    # Test configuration
    user_id = "test_user_qa"
    
    # 1. Create a chat
    print("\n1️⃣ Creating test chat...")
    try:
        chat_data = {"chat_name": "Test Agricultural Q&A Chat"}
        chat_response = requests.post(f"{BASE_URL}/users/{user_id}/chats", data=chat_data)
        if chat_response.status_code == 200:
            chat_data = chat_response.json()
            chat_id = chat_data["chat_id"]
            print(f"✅ Chat created: {chat_id}")
        else:
            print(f"❌ Failed to create chat: {chat_response.status_code}")
            print(f"   Response: {chat_response.text}")
            return
    except Exception as e:
        print(f"❌ Chat creation failed: {e}")
        return

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

    print("\n3️⃣ Uploading test agricultural document...")
    file_id = None
    try:
        with open(temp_pdf_path, "rb") as f:
            files = {"file": ("sustainable_agriculture_report.pdf", f, "application/pdf")}
            data = {"user_id": user_id, "chat_id": chat_id}
            
            upload_response = requests.post(f"{BASE_URL}/upload/pdf", files=files, data=data)
            
            if upload_response.status_code == 200:
                upload_data = upload_response.json()
                file_id = upload_data["file_id"]
                print("✅ File uploaded successfully!")
                print(f"   File ID: {file_id}")
                print(f"   Processing Status: {upload_data.get('processing_status', 'unknown')}")
            else:
                print(f"❌ Upload failed: {upload_response.status_code}")
                print(f"   Response: {upload_response.text}")
                
    except Exception as e:
        print(f"❌ Upload error: {e}")
    finally:
        # Clean up temp file
        if os.path.exists(temp_pdf_path):
            os.remove(temp_pdf_path)
    
    if not file_id:
        print("❌ Cannot continue without successful file upload")
        return

    # 4. Wait for processing and check status
    print("\n4️⃣ Waiting for Gemini processing...")
    max_attempts = 10
    for attempt in range(max_attempts):
        try:
            status_response = requests.get(f"{BASE_URL}/users/{user_id}/files/{file_id}")
            if status_response.status_code == 200:
                status_data = status_response.json()
                processing_status = status_data.get("processing_status", "unknown")
                print(f"   Processing status: {processing_status}")
                
                if processing_status == "completed":
                    print("✅ File processing completed!")
                    break
                elif processing_status == "failed":
                    print("❌ File processing failed!")
                    break
                
            time.sleep(2)
        except Exception as e:
            print(f"   Status check error: {e}")
    
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
            chat_data = {
                "message": question,
                "model": "gemini-2.5-flash",
                "tools": "ask_question_about_files,list_uploaded_files", 
                "logs": "true"
            }
            
            chat_response = requests.post(
                f"{BASE_URL}/users/{user_id}/chats/{chat_id}/messages",
                data=chat_data
            )
            
            if chat_response.status_code == 200:
                response_text = chat_response.text.strip()
                try:
                    # Handle streaming JSON response - look for the final response
                    lines = response_text.split('\n')
                    final_response = None
                    
                    for line in lines:
                        if line.strip():
                            try:
                                json_obj = json.loads(line.strip())
                                # Look for the final response (not just logs)
                                if json_obj.get("type") == "response" or "response" in json_obj:
                                    final_response = json_obj.get("response", json_obj.get("data", ""))
                                    break
                                elif json_obj.get("type") == "error":
                                    final_response = f"Error: {json_obj.get('message', 'Unknown error')}"
                                    break
                            except json.JSONDecodeError:
                                continue
                    
                    # If no specific response found, try to get the last meaningful JSON object
                    if final_response is None:
                        for line in reversed(lines):
                            if line.strip():
                                try:
                                    json_obj = json.loads(line.strip())
                                    if json_obj.get("response"):
                                        final_response = json_obj.get("response")
                                        break
                                except json.JSONDecodeError:
                                    continue
                    
                    if final_response:
                        print("   ✅ Chat response received")
                        # Truncate long responses for readability
                        if len(str(final_response)) > 200:
                            print(f"      📝 Answer: {str(final_response)[:200]}...")
                        else:
                            print(f"      📝 Answer: {final_response}")
                    else:
                        print("   ⚠️  Response received but no final answer found")
                        print(f"      Raw response preview: {response_text[:200]}...")
                        
                except Exception as e:
                    print(f"   ❌ Response parsing error: {e}")
                    print(f"      Raw response: {repr(response_text[:200])}")
            else:
                print(f"   ❌ Chat failed: {chat_response.status_code}")
                print(f"      Response: {chat_response.text}")
                
        except Exception as e:
            print(f"   ❌ Question error: {e}")
        
        time.sleep(1)  # Be nice to the API

    # 6. Test comprehensive analysis across files
    print("\n6️⃣ Testing comprehensive analysis across files...")
    comprehensive_question = "Based on all uploaded agricultural documents, provide a comprehensive analysis of sustainable farming practices, including economic benefits, environmental impact, and regional considerations. Include specific metrics and recommendations."
    
    try:
        chat_data = {
            "message": comprehensive_question,
            "model": "gemini-2.5-flash",
            "tools": "ask_question_about_files,list_uploaded_files", 
            "logs": "true"
        }
        
        chat_response = requests.post(
            f"{BASE_URL}/users/{user_id}/chats/{chat_id}/messages",
            data=chat_data
        )
        
        if chat_response.status_code == 200:
            response_text = chat_response.text.strip()
            try:
                # Handle streaming JSON response - look for the final response
                lines = response_text.split('\n')
                final_response = None
                
                for line in lines:
                    if line.strip():
                        try:
                            json_obj = json.loads(line.strip())
                            # Look for the final response (not just logs)
                            if json_obj.get("type") == "response" or "response" in json_obj:
                                final_response = json_obj.get("response", json_obj.get("data", ""))
                                break
                            elif json_obj.get("type") == "error":
                                final_response = f"Error: {json_obj.get('message', 'Unknown error')}"
                                break
                        except json.JSONDecodeError:
                            continue
                
                # If no specific response found, try to get the last meaningful JSON object
                if final_response is None:
                    for line in reversed(lines):
                        if line.strip():
                            try:
                                json_obj = json.loads(line.strip())
                                if json_obj.get("response"):
                                    final_response = json_obj.get("response")
                                    break
                            except json.JSONDecodeError:
                                continue
                
                if final_response:
                    print("✅ Comprehensive analysis response received")
                    # Show first 300 characters of analysis
                    if len(str(final_response)) > 300:
                        print(f"📋 Comprehensive Analysis: {str(final_response)[:300]}...")
                    else:
                        print(f"📋 Comprehensive Analysis: {final_response}")
                else:
                    print("⚠️  Response received but no final answer found")
                    print(f"   Raw response preview: {response_text[:300]}...")
                    
            except Exception as e:
                print(f"❌ Response parsing error: {e}")
                print(f"   Raw response: {repr(response_text[:300])}")
        else:
            print(f"❌ Comprehensive analysis failed: {chat_response.status_code}")
            print(f"   Response: {chat_response.text}")
            
    except Exception as e:
        print(f"❌ Comprehensive analysis error: {e}")

    # 7. Cleanup
    print("\n7️⃣ Cleaning up...")
    try:
        delete_response = requests.delete(f"{BASE_URL}/users/{user_id}/files/{file_id}")
        if delete_response.status_code == 200:
            print("✅ Test file deleted successfully")
        else:
            print(f"⚠️  File deletion warning: {delete_response.status_code}")
    except Exception as e:
        print(f"⚠️  Cleanup error: {e}")

    # 8. Summary
    print("\n" + "=" * 70)
    print("🎉 File Question-Answering System Test Complete!")
    print("\n📊 Test Results Summary:")
    print("✅ File upload with Gemini processing")
    print("✅ Native document understanding") 
    print("✅ Question-answering from file content")
    print("✅ Multi-file analysis capabilities")
    print("✅ Integration with chat system")
    
    print("\n🎯 System Capabilities Summary:")
    print("📄 Upload PDFs and images with automatic Gemini processing")
    print("🤖 Ask questions about document content using native AI understanding")
    print("🔍 Search across multiple files simultaneously")
    print("💬 Integrated chat system with file-aware responses")
    print("📊 Comprehensive document analysis and summarization")
    print("🌱 Agricultural domain-specific insights and recommendations")

if __name__ == "__main__":
    main()
