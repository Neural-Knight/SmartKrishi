# SmartKrishi API Documentation

## Overview ✨ ENHANCED

The SmartKrishi API is a multi-tenant FastAPI application that provides AI-powered agricultural advice with **advanced document processing**, **code execution**, and **data analysis** capabilities. The API supports chat-based interactions, enhanced file uploads (PDFs, images, DOCX, XLSX, CSV), tool integration with code execution, URL context, and streaming responses.

**New Features:**
- 📈 **CSV Data Analysis**: Upload CSV files for comprehensive agricultural data analysis and visualization
- 📊 **XLSX Data Analysis**: Upload spreadsheets for automated data analysis and visualization
- 📄 **DOCX Processing**: Word document analysis with comprehensive text and table extraction  
- 💻 **Code Execution**: Dynamic Python code execution for data analysis and chart generation
- 🌐 **URL Context**: Reference external agricultural resources and research papers
- 🔄 **Multiturn Analysis**: Chain analysis steps across conversation turns
- 🌍 **Ngrok Support**: Dynamic API base URL for seamless remote access via ngrok

**Base URL**: 
- Local: `http://localhost:8080` (updated port)
- Ngrok: Dynamic (automatically detected by frontend)
- External: Accessible on all network interfaces (`0.0.0.0:8080`)

---

## Authentication

Currently, the API uses simple user identification via `user_id` parameters. No authentication tokens are required.

---

## Supported File Formats ✨ COMPREHENSIVE

| Format | Icon | Extension | Processing Capabilities | Agricultural Use Cases |
|--------|------|-----------|------------------------|----------------------|
| **PDF** | 📄 | `.pdf` | Text extraction, document analysis | Research papers, manuals, reports |
| **Images** | 🖼️ | `.jpg`, `.png`, `.gif` | Visual analysis, object detection | Crop photos, field conditions, equipment |
| **Word Documents** | 📝 | `.docx` | Text & table extraction, structure analysis | Reports, documentation, research |
| **Excel Spreadsheets** | 📊 | `.xlsx` | Data analysis, visualization, statistics | Financial data, yield records, planning |
| **CSV Data** | 📈 | `.csv` | Comprehensive data analysis, pattern recognition | Sensor data, market prices, historical records |

**Key Features**:
- All files processed with Gemini's code execution for advanced analysis
- Automatic agricultural data pattern recognition
- Real-time visualization generation
- Data quality assessment and recommendations
- Background processing with status tracking

---

## Core Endpoints

### Frontend

#### `GET /`
**Description**: Serve the frontend HTML application  
**Response**: HTML file for the web interface

---

## Chat Management

### `POST /users/{user_id}/chats`
**Description**: Create a new chat for a user

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `chat_name` (form): Name for the new chat

**Response**:
```json
{
  "chat_id": "uuid-string",
  "user_id": "string",
  "chat_name": "string"
}
```

**Example**:
```bash
curl -X POST "http://localhost:8080/users/alice/chats" \
  -F "chat_name=My Farm Discussion"
```

---

### `GET /users/{user_id}/chats`
**Description**: Get all chats for a user

**Parameters**:
- `user_id` (path): Unique identifier for the user

**Response**:
```json
{
  "user_id": "string",
  "chats": [
    {
      "chat_id": "uuid-string",
      "chat_name": "string",
      "created_at": "timestamp",
      "message_count": 0
    }
  ]
}
```

---

### `GET /users/{user_id}/chats/{chat_id}`
**Description**: Get specific chat details

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `chat_id` (path): Unique identifier for the chat

**Response**:
```json
{
  "user_id": "string",
  "chat_id": "string",
  "chat_name": "string",
  "created_at": "timestamp",
  "message_count": 0
}
```

**Error Responses**:
- `404`: Chat not found

---

### `DELETE /users/{user_id}/chats/{chat_id}`
**Description**: Delete a chat and all its messages

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `chat_id` (path): Unique identifier for the chat

**Response**:
```json
{
  "message": "Chat deleted successfully",
  "chat_id": "string"
}
```

**Error Responses**:
- `404`: Chat not found

---

### `GET /users/{user_id}/chats/{chat_id}/messages`
**Description**: Get messages for a specific chat

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `chat_id` (path): Unique identifier for the chat
- `limit` (query, optional): Maximum number of messages to return (default: 100)

**Response**:
```json
{
  "user_id": "string",
  "chat_id": "string",
  "messages": [
    {
      "id": "uuid-string",
      "role": "user|assistant",
      "msg": "string",
      "timestamp": "ISO-datetime"
    }
  ]
}
```

---

### `POST /users/{user_id}/chats/{chat_id}/messages`
**Description**: Send a message to a specific chat and get streaming response

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `chat_id` (path): Unique identifier for the chat
- `message` (form): The user's message/question
- `model` (form, optional): AI model to use (default: "gemini-2.5-flash")
- `tools` (form, optional): Comma-separated list of tool names to include
- `logs` (form, optional): Whether to include detailed logs (default: false)

**Response**: Streaming JSON with various event types (see Streaming Response section)

---

## AI Query Endpoints

### `POST /ask`
**Description**: Ask a question in a specific chat with optional detailed logging (non-streaming)

**Parameters**:
- `q` (form): The question to ask
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier
- `logs` (form, optional): Include detailed processing logs (default: false)

**Response**:
```json
{
  "answer": "string",
  "approved": true,
  "confidence": 0.95,
  "issues": [],
  "tools": ["tool1", "tool2"],
  "user_id": "string",
  "chat_id": "string",
  "detailed_logs": {...},  // If logs=true
  "grounding": {...}       // If logs=true
}
```

---

### `POST /ask_stream` ✨ ENHANCED
**Description**: Ask a question with streaming response including code execution and URL context

**Parameters**:
- `q` (form): The question to ask
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier
- `include_tools` (form, optional): Comma-separated list of tools to use
- `logs` (form, optional): Include detailed logs (default: false)

**Response**: Server-Sent Events (SSE) stream with JSON objects

**Enhanced Streaming Event Types**:
```json
{"type": "log", "stage": "initialization", "data": {...}}
{"type": "plan", "plan": {...}, "raw_response": "string"}
{"type": "tool_call", "tool": "string", "args": "...", "result": {...}}
{"type": "thinking", "content": "string"}
{"type": "response_chunk", "content": "string"}
{"type": "code_execution", "code": "string", "result": "string", "output": "..."} 
{"type": "url_context", "urls": [...], "content": "string"}
{"type": "grounding_web_search_queries", "queries": [...]}
{"type": "grounding_chunks", "sources": [...]}
{"type": "response", "response": "string", "grounding_metadata": {...}}
{"type": "visualization", "image_data": "base64_string", "description": "string"}
{"type": "end"}
```

**New Capabilities**:
- ✨ **Code Execution**: Automatically executes Python code for data analysis
- ✨ **URL Context**: References external web resources in responses
- ✨ **File Integration**: Automatically includes uploaded file context
- ✨ **Visualizations**: Generates charts and graphs from data analysis
- ✨ **Multiturn Analysis**: Chains analysis steps across conversation

---

## File Management

### `POST /upload/pdf`
**Description**: Upload a PDF file for processing

**Parameters**:
- `file` (multipart): PDF file to upload
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier

**Response**:
```json
{
  "file_id": "uuid-string",
  "stored_path": "string",
  "user_id": "string",
  "chat_id": "string",
  "filename": "string",
  "status": "uploaded",
  "processing_status": "processing|completed|failed",
  "message": "File uploaded successfully. Processing in background..."
}
```

**Error Responses**:
- `500`: Upload failed

---

### `POST /upload/image` ✨ ENHANCED
**Description**: Upload an image file for comprehensive agricultural vision analysis

**Parameters**:
- `file` (multipart): Image file to upload (JPG, JPEG, PNG, GIF)
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier

**Response**:
```json
{
  "file_id": "uuid-string",
  "stored_path": "string",
  "user_id": "string",
  "chat_id": "string",
  "filename": "string",
  "status": "uploaded",
  "processing_status": "processing",
  "message": "Image uploaded successfully. Processing with vision analysis..."
}
```

**Features**:
- Uses Gemini's advanced vision capabilities for comprehensive image analysis
- Provides detailed agricultural-focused analysis (crops, equipment, land conditions)
- Identifies pests, diseases, and agricultural issues
- Integrates with chat conversations for contextual image discussions
- Supports high-resolution agricultural photography
- Enables follow-up questions about uploaded images

**Agricultural Use Cases**:
- Crop health assessment and disease identification
- Field condition monitoring and soil analysis
- Equipment inspection and maintenance needs
- Pest detection and treatment recommendations
- Harvest readiness evaluation
- Land use planning and mapping

---

### `POST /upload/docx` ✨ NEW
**Description**: Upload a DOCX file for advanced document analysis with code execution

**Parameters**:
- `file` (multipart): DOCX file to upload (Word document)
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier

**Response**:
```json
{
  "file_id": "uuid-string",
  "stored_path": "string",
  "user_id": "string",
  "chat_id": "string",
  "filename": "string",
  "status": "uploaded",
  "processing_status": "processing",
  "message": "DOCX file uploaded successfully. Processing with code execution..."
}
```

**Features**:
- Uses Gemini's code execution with python-docx library
- Extracts text, tables, and structured content
- Analyzes document themes and agricultural relevance
- Provides comprehensive document insights

---

### `POST /upload/xlsx` ✨ NEW
**Description**: Upload an XLSX file for data analysis and visualization

**Parameters**:
- `file` (multipart): XLSX file to upload (Excel spreadsheet)
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier

**Response**:
```json
{
  "file_id": "uuid-string",
  "stored_path": "string",
  "user_id": "string",
  "chat_id": "string",
  "filename": "string",
  "status": "uploaded",
  "processing_status": "processing",
  "message": "XLSX file uploaded successfully. Processing with data analysis..."
}
```

**Features**:
- Uses Gemini's code execution with pandas and openpyxl
- Analyzes data structure, patterns, and statistics
- Generates visualizations with matplotlib
- Identifies agricultural data trends and insights
- Creates summary statistics and data profiles

---

### `POST /upload/csv` ✨ NEW
**Description**: Upload a CSV file for comprehensive data analysis and visualization

**Parameters**:
- `file` (multipart): CSV file to upload (comma-separated values)
- `user_id` (form, optional): User identifier (auto-generated if not provided)
- `chat_id` (form): Chat identifier

**Response**:
```json
{
  "file_id": "uuid-string",
  "stored_path": "string",
  "user_id": "string",
  "chat_id": "string",
  "filename": "string",
  "status": "uploaded",
  "processing_status": "processing",
  "message": "CSV file uploaded successfully. Processing with data analysis..."
}
```

**Features**:
- Uses Gemini's code execution with pandas for comprehensive analysis
- Analyzes data structure, columns, data types, and data quality
- Identifies agricultural data patterns (crop yields, weather data, market prices, etc.)
- Detects missing values, outliers, and data quality issues
- Generates summary statistics and data insights
- Creates visualizations for data exploration
- Provides data-driven recommendations for agricultural decisions
- Supports common CSV formats with automatic delimiter detection

**Agricultural Use Cases**:
- Crop yield analysis across seasons and regions
- Weather data correlation with crop performance  
- Market price trend analysis and forecasting
- Farm operation cost tracking and optimization
- Soil analysis data interpretation
- Equipment usage and maintenance scheduling

---

### `GET /file/{file_id}/details` ✨ NEW
**Description**: Get comprehensive analysis details for an uploaded file

**Parameters**:
- `file_id` (path): Unique identifier for the file

**Response**:
```json
{
  "file_id": "uuid-string",
  "filename": "string",
  "analysis": "Detailed AI analysis of the file contents...",
  "metadata": {
    "upload_date": "2024-01-15T10:30:00Z",
    "file_size": 1024000,
    "file_type": "pdf|docx|xlsx|csv|image",
    "processing_status": "completed|processing|failed"
  }
}
```

**Features**:
- Provides detailed AI-generated analysis of file contents
- Includes comprehensive metadata about the file
- Returns structured analysis with agricultural insights
- Supports all file types (PDF, DOCX, XLSX, CSV, images)

**Error Responses**:
- `404`: File not found
- `500`: Analysis retrieval failed

**Usage Example**:
```javascript
// Frontend JavaScript example
async function getFileAnalysis(fileId) {
    const response = await fetch(`/file/${fileId}/details`);
    const data = await response.json();
    console.log(data.analysis);
}
```

---

### `GET /users/{user_id}/chats/{chat_id}/files`
**Description**: List all files uploaded to a specific chat

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `chat_id` (path): Unique identifier for the chat

**Response**:
```json
{
  "user_id": "string",
  "chat_id": "string",
  "files": [
    {
      "file_id": "uuid-string",
      "original_filename": "string",
      "file_type": "pdf|image|docx|xlsx",
      "processing_status": "processing|completed|failed",
      "upload_time": "ISO-datetime",
      "summary": "string"
    }
  ],
  "total_count": 0
}
```

---

### `GET /users/{user_id}/files`
**Description**: List all files for a user across all chats

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `limit` (query, optional): Maximum number of files to return (default: 50)

**Response**:
```json
{
  "user_id": "string",
  "files": [...],  // Same structure as chat files
  "total_count": 0
}
```

---

### `GET /users/{user_id}/files/{file_id}`
**Description**: Get detailed information about a specific file

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `file_id` (path): Unique identifier for the file

**Response**:
```json
{
  "file_id": "string",
  "user_id": "string",
  "chat_id": "string",
  "original_filename": "string",
  "stored_path": "string",
  "file_type": "pdf|image",
  "file_size": 1024,
  "processing_status": "processing|completed|failed",
  "upload_time": "ISO-datetime",
  "summary": "string",
  "metadata": {...}
}
```

**Error Responses**:
- `404`: File not found

---

### `DELETE /users/{user_id}/files/{file_id}`
**Description**: Delete a file and its metadata

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `file_id` (path): Unique identifier for the file

**Response**:
```json
{
  "message": "File deleted successfully",
  "file_id": "string"
}
```

**Error Responses**:
- `404`: File not found

---

### `GET /users/{user_id}/files/search`
**Description**: Search files by filename or content

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `query` (query): Search query string
- `chat_id` (query, optional): Limit search to specific chat

**Response**:
```json
{
  "user_id": "string",
  "chat_id": "string",
  "query": "string",
  "files": [...],  // Matching files
  "total_matches": 0
}
```

---

## Tool Management

### `GET /tools`
**Description**: List all available tool names

**Response**:
```json
{
  "tools": ["weather_api", "soil_api", "market_api", "chat_history"]
}
```

---

### `GET /users/{user_id}/tools`
**Description**: Get current tool preferences for a user

**Parameters**:
- `user_id` (path): Unique identifier for the user

**Response**:
```json
{
  "user_id": "string",
  "include_tools": ["weather_api", "soil_api"]
}
```

---

### `POST /users/{user_id}/tools`
**Description**: Set tool preferences for a user

**Parameters**:
- `user_id` (path): Unique identifier for the user
- `include_tools` (JSON body): Array of tool names to include

**Request Body**:
```json
["weather_api", "soil_api", "market_api"]
```

**Response**:
```json
{
  "user_id": "string",
  "include_tools": ["weather_api", "soil_api", "market_api"]
}
```

---

## Available Tools

The system includes several built-in tools:

1. **weather_api**: Get weather information for locations
2. **soil_api**: Retrieve soil data and analysis
3. **market_api**: Access agricultural market prices and trends
4. **chat_history**: Search through chat message history

---

## Error Handling

All endpoints return appropriate HTTP status codes:

- `200`: Success
- `404`: Resource not found
- `500`: Internal server error

Error responses follow this format:
```json
{
  "error": "Error description"
}
```

---

## Streaming Response Details

The `/ask_stream` endpoint returns Server-Sent Events with the following event types:

### Log Events
```json
{"type": "log", "stage": "initialization|planner_start|tools_start|agent_start|complete", "message": "string", "data": {...}}
```

### Plan Events
```json
{"type": "plan", "plan": {"primary_intent": "string", "tools_needed": [], "location": "string", "crop": "string"}, "raw_response": "string"}
```

### Tool Execution Events
```json
{"type": "tool_call", "tool": "tool_name", "args": "arguments", "result": {...}}
```

### AI Thinking Events
```json
{"type": "thinking", "content": "AI reasoning text"}
```

### Response Streaming Events
```json
{"type": "response_chunk", "content": "partial response text"}
```

### Grounding Events
```json
{"type": "grounding_web_search_queries", "queries": ["query1", "query2"]}
{"type": "grounding_chunks", "sources": [{"uri": "string", "title": "string"}]}
```

### Final Response Event
```json
{"type": "response", "response": "complete response", "grounding_metadata": {...}}
```

### End Event
```json
{"type": "end"}
```

---

## Example Usage

### Creating a Chat and Asking a Question

```bash
# 1. Create a new chat
curl -X POST "http://localhost:8080/users/farmer123/chats" \
  -F "chat_name=Tomato Farming Help"

# Response: {"chat_id": "abc123", "user_id": "farmer123", "chat_name": "Tomato Farming Help"}

# 2. Ask a question
curl -X POST "http://localhost:8080/ask_stream" \
  -F "q=What's the best time to plant tomatoes in California?" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123" \
  -F "include_tools=weather_api,soil_api" \
  -F "logs=true"
```

### Uploading and Querying Files ✨ ENHANCED

```bash
# 1. Upload a PDF
curl -X POST "http://localhost:8080/upload/pdf" \
  -F "file=@farming_guide.pdf" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 2. Upload a DOCX document for analysis
curl -X POST "http://localhost:8080/upload/docx" \
  -F "file=@research_paper.docx" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 3. Upload an XLSX file for data analysis
curl -X POST "http://localhost:8080/upload/xlsx" \
  -F "file=@yield_data.xlsx" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 4. Upload a CSV file for comprehensive data analysis ✨ NEW
curl -X POST "http://localhost:8080/upload/csv" \
  -F "file=@crop_yields.csv" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 5. List files in chat
curl "http://localhost:8080/users/farmer123/chats/abc123/files"

# 6. Ask for CSV data analysis with code execution
curl -X POST "http://localhost:8080/ask_stream" \
  -F "q=Analyze my CSV data and create visualizations showing yield trends, identify outliers, and provide recommendations" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 7. Ask for XLSX data analysis
curl -X POST "http://localhost:8080/ask_stream" \
  -F "q=Analyze my yield data and create visualizations showing trends over time" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 8. Ask for document analysis
curl -X POST "http://localhost:8080/ask_stream" \
  -F "q=Summarize the key findings from the uploaded research paper" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"

# 9. Combine file analysis with web research
curl -X POST "http://localhost:8080/ask_stream" \
  -F "q=Compare my yield data with USDA national averages from https://usda.gov/crops" \
  -F "user_id=farmer123" \
  -F "chat_id=abc123"
```

---

## Recent Updates ✨ ENHANCED

### CSV File Upload Support (Latest)
- **NEW**: Complete CSV file upload and analysis capabilities
- Comprehensive data analysis with pandas and Gemini code execution
- Agricultural data pattern recognition (yields, weather, market prices)
- Data quality assessment (missing values, outliers, inconsistencies)
- Automated visualization generation for data exploration
- Intelligent recommendations based on agricultural data insights

### API Configuration Improvements
- **Port Update**: Default port changed from 8000 to 8080 for better compatibility
- **Ngrok Support**: Dynamic API base URL detection for seamless remote access
- **Network Access**: Server now listens on all interfaces (0.0.0.0) for external access
- **CORS Configuration**: Enhanced cross-origin support for frontend-backend communication

### Enhanced File Processing Pipeline
- **Unified Processing**: All file types (PDF, Images, DOCX, XLSX, CSV) use consistent upload endpoints
- **Background Processing**: Files are processed asynchronously with status tracking
- **Metadata Storage**: Complete file metadata and processing status persistence
- **Error Handling**: Robust error handling and user feedback for upload failures

### Frontend Improvements
- **Dynamic API Base**: Frontend automatically detects API base URL (localhost or ngrok)
- **File Type Support**: Complete UI support for all file formats with appropriate icons
- **Upload Feedback**: Enhanced user feedback for file upload status and errors
- **Mobile Compatibility**: Responsive design for cross-device accessibility

---

## Development Notes ✨ ENHANCED

- The API automatically includes uploaded file summaries in AI responses
- **Code execution** runs in Gemini's secure environment with 30-second timeout
- **URL context** enables referencing external agricultural resources
- **DOCX files** are processed with python-docx for comprehensive analysis
- **XLSX files** are analyzed with pandas/openpyxl for data insights and visualizations
- **CSV files** are processed with pandas for comprehensive data analysis and agricultural insights
- Tool preferences are stored in memory and reset on server restart
- File processing happens in the background with status tracking
- Streaming responses support real-time interaction with code execution results
- CORS is enabled for all origins (should be restricted in production)
- **Multiturn conversations** maintain context across analysis steps

---

## Dependencies ✨ UPDATED

- FastAPI
- Google Generative AI (Gemini) with code execution and URL context
- **python-docx** - Word document processing
- **openpyxl** - Excel file processing  
- **pandas** - CSV and data analysis (available in Gemini environment)
- **matplotlib** - Chart generation (available in Gemini environment)
- SQLite for data storage
- ChromaDB for vector storage
- Various agricultural data APIs
