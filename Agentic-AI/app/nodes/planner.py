import json, logging
import os
from google import genai
from ..state import State
from ..file_manager import get_chat_files

MODEL = "gemini-2.5-flash"
log   = logging.getLogger("Planner")

def planner_node(state: State) -> State:
    # Get information about uploaded files for better planning
    files_info = ""
    try:
        chat_files = get_chat_files(state.chat_id, state.user_id)
        if chat_files:
            files_info = f"\n\nAvailable uploaded files ({len(chat_files)} total):\n"
            for file_info in chat_files:
                files_info += f"- {file_info['original_filename']} ({file_info['file_type']}, {file_info['processing_status']})\n"
    except Exception as e:
        log.warning("Failed to get file info for planning: %s", e)
        files_info = ""
    
    # Detailed tool descriptions for planning
    tool_descriptions = """
Available tools and their capabilities:
- weather_api: Get current weather conditions, forecasts, and historical weather data for any location
- soil_api: Get soil analysis, pH levels, nutrient content, and soil health recommendations
- market_api: Get current crop prices, market trends, and commodity information
- chat_history: Access previous conversations and advice given to the user
- get_pdf_content: Extract and analyze text content from uploaded PDF documents
- get_image_analysis: Analyze uploaded images for plant diseases, pests, crop conditions, or equipment
- list_uploaded_files: List all files uploaded by the user in this chat
- search_user_files: Search through user's uploaded files for specific content
- ask_question_about_files: Answer questions based on the content of uploaded files

Your role is PLANNING ONLY - do not execute any tools, just plan which ones would be helpful.
"""
    
    prompt = f"""You are an agricultural planning assistant. Your job is to analyze the user's query and create a plan for how to help them, but NOT to execute any tools.

{tool_descriptions}
{files_info}

User Query: "{state.user_query}"

Analyze this query and determine what information would be needed to provide a helpful response. Consider:
- What type of agricultural advice is being requested?
- What data sources would be most relevant?
- Are there any uploaded files that might contain relevant information?
- What location-specific information might be needed?

Example planning scenarios:
- Weather question → ["weather_api"] 
- Crop disease in uploaded image → ["get_image_analysis", "weather_api"]
- Market prices for corn → ["market_api"]
- Soil health advice → ["soil_api", "weather_api"]
- Question about uploaded PDF → ["ask_question_about_files"]
- General farming advice → ["weather_api", "soil_api", "chat_history"]

Return a JSON plan with these keys:
- primary_intent: The main goal (e.g., "weather_forecast", "crop_advice", "market_analysis", "file_analysis", etc.)
- tools_needed: Array of tool names that would be helpful to answer this query
- location: Any location mentioned or "unknown" if not specified
- crop: Any crop mentioned or "general" if not specified
- reasoning: Brief explanation of why these tools were chosen

Remember: You are ONLY planning - the actual tool execution will happen later."""
    
    # initialize client with API key
    client = genai.Client(api_key=os.getenv('GOOGLE_API_KEY'))
    chat = client.chats.create(model=MODEL)
    rsp  = chat.send_message(prompt)
    # parse JSON plan safely
    text = rsp.text or ""
    try:
        state.plan = json.loads(text)
    except Exception:
        # Default plan - include file tools if files are available
        default_tools = ["weather_api"]
        if files_info:
            default_tools.append("ask_question_about_files")
        state.plan = {"primary_intent":"advise",
                      "tools_needed":default_tools,
                      "location":"unknown",
                      "crop":"general",
                      "reasoning":"Default fallback plan"}
    log.info("Plan for %s: %s", state.user_id, state.plan)
    return state
