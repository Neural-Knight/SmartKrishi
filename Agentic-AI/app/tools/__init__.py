from .weather import weather_api
from .market  import market_api
from .soil    import soil_api
from .chat_history import chat_history_tool
from .file_tools import get_pdf_content, get_image_analysis, list_uploaded_files, search_user_files, ask_question_about_files

TOOLS = {
    "weather_api": weather_api,
    "market_api" : market_api,
    "soil_api"   : soil_api,
    "chat_history": chat_history_tool,
    "get_pdf_content": get_pdf_content,
    "get_image_analysis": get_image_analysis,
    "list_uploaded_files": list_uploaded_files,
    "search_user_files": search_user_files,
    "ask_question_about_files": ask_question_about_files,
}
