"""
Tools for Gemini to access and interact with uploaded files
"""
import os
import logging
from typing import Dict, Any, List, Optional
from google import genai
from app.file_manager import get_file_metadata, get_chat_files, search_files
from app.media import PDF_COL

log = logging.getLogger("FileTools")

def get_pdf_content(file_id: str, user_id: str, query: Optional[str] = None) -> Dict[str, Any]:
    """
    Tool for Gemini to retrieve PDF content by file ID or ask questions about the document.
    
    Args:
        file_id: The unique file identifier
        user_id: The user who owns the file
        query: Optional search query or question about the document
        
    Returns:
        Dict with PDF content, analysis, or answers to questions
    """
    try:
        # Get file metadata
        file_info = get_file_metadata(file_id, user_id)
        if not file_info:
            return {"error": "File not found or access denied"}
        
        if file_info["file_type"] != "pdf":
            return {"error": "File is not a PDF"}
        
        # If user asked a specific question, use Gemini to answer from the document
        if query and file_info.get("gemini_file_id"):
            try:
                from google import genai
                api_key = os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_API_KEY")
                
                if api_key:
                    client = genai.Client(api_key=api_key)
                    
                    # Get the file reference from Gemini
                    gemini_file = client.files.get(name=file_info["gemini_file_id"])
                    
                    # Ask the question directly to the document
                    response = client.models.generate_content(
                        model="gemini-2.5-flash",
                        contents=[
                            gemini_file,
                            f"""Based on the content of this document, please answer the following question:

{query}

Provide a comprehensive answer based on the document content. If the information is not available in the document, please state that clearly."""
                        ]
                    )
                    
                    answer = ""
                    if hasattr(response, 'text'):
                        answer = response.text
                    elif (hasattr(response, 'candidates') and response.candidates and 
                          len(response.candidates) > 0):
                        candidate = response.candidates[0]
                        if (hasattr(candidate, 'content') and candidate.content and 
                            hasattr(candidate.content, 'parts') and candidate.content.parts):
                            answer = candidate.content.parts[0].text
                    
                    return {
                        "file_id": file_id,
                        "filename": file_info["original_filename"],
                        "question": query,
                        "answer": answer,
                        "content_type": "gemini_qa",
                        "summary": file_info.get("summary", ""),
                        "file_size": file_info["file_size"]
                    }
                    
            except Exception as e:
                log.error(f"Gemini document Q&A failed: {e}")
                # Fallback to ChromaDB search
        
        # Fallback to ChromaDB search/retrieval
        try:
            if query:
                # Search for relevant sections
                results = PDF_COL.query(
                    query_texts=[query],
                    where={"user": user_id, "file_id": file_id},
                    n_results=3
                )
                
                if results["documents"] and results["documents"][0]:
                    content = "\n\n---\n\n".join(results["documents"][0])
                    return {
                        "file_id": file_id,
                        "filename": file_info["original_filename"],
                        "content": content,
                        "content_type": "search_results",
                        "query": query,
                        "summary": file_info.get("summary", "No summary available"),
                        "file_size": file_info["file_size"]
                    }
                else:
                    return {"error": "No relevant content found for query"}
            else:
                # Get full content
                results = PDF_COL.get(
                    where={"user": user_id, "file_id": file_id}
                )
                
                if results["documents"]:
                    content = results["documents"][0]
                    return {
                        "file_id": file_id,
                        "filename": file_info["original_filename"],
                        "content": content,
                        "content_type": "full_document",
                        "summary": file_info.get("summary", "No summary available"),
                        "file_size": file_info["file_size"]
                    }
                else:
                    return {"error": "PDF content not found in database"}
                    
        except Exception as e:
            log.error(f"ChromaDB query failed: {e}")
            # Final fallback to content preview
            if file_info.get("content_preview"):
                return {
                    "file_id": file_id,
                    "filename": file_info["original_filename"],
                    "content": file_info["content_preview"],
                    "content_type": "preview",
                    "summary": file_info.get("summary", "No summary available"),
                    "note": "Full content unavailable, showing preview"
                }
            else:
                return {"error": "PDF content not accessible"}
                
    except Exception as e:
        log.error(f"Error retrieving PDF content: {e}")
        return {"error": f"Failed to retrieve PDF content: {str(e)}"}

def get_image_analysis(file_id: str, user_id: str, question: Optional[str] = None) -> Dict[str, Any]:
    """
    Tool for Gemini to analyze images by file ID and answer specific questions.
    
    Args:
        file_id: The unique file identifier
        user_id: The user who owns the file
        question: Optional specific question about the image
        
    Returns:
        Dict with image analysis and answers to questions
    """
    try:
        # Get file metadata
        file_info = get_file_metadata(file_id, user_id)
        if not file_info:
            return {"error": "File not found or access denied"}
        
        if file_info["file_type"] != "image":
            return {"error": "File is not an image"}
        
        # If user asked a specific question and we have Gemini file, analyze directly
        if question and file_info.get("gemini_file_id"):
            try:
                from google import genai
                api_key = os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_API_KEY")
                
                if api_key:
                    client = genai.Client(api_key=api_key)
                    
                    # Get the file reference from Gemini
                    gemini_file = client.files.get(name=file_info["gemini_file_id"])
                    
                    # Ask the question about the image
                    response = client.models.generate_content(
                        model="gemini-2.5-flash",
                        contents=[
                            gemini_file,
                            f"""Please analyze this image and answer the following question:

{question}

Provide a detailed answer based on what you can see in the image. Include specific details about any text, objects, charts, data, or agricultural elements visible."""
                        ]
                    )
                    
                    answer = ""
                    if hasattr(response, 'text'):
                        answer = response.text
                    elif (hasattr(response, 'candidates') and response.candidates and 
                          len(response.candidates) > 0):
                        candidate = response.candidates[0]
                        if (hasattr(candidate, 'content') and candidate.content and 
                            hasattr(candidate.content, 'parts') and candidate.content.parts):
                            answer = candidate.content.parts[0].text
                    
                    return {
                        "file_id": file_id,
                        "filename": file_info["original_filename"],
                        "question": question,
                        "analysis": answer,
                        "analysis_type": "gemini_qa",
                        "summary": file_info.get("summary", ""),
                        "file_size": file_info["file_size"]
                    }
                    
            except Exception as e:
                log.error(f"Gemini image Q&A failed: {e}")
        
        # Return stored summary/analysis as fallback
        return {
            "file_id": file_id,
            "filename": file_info["original_filename"],
            "analysis": file_info.get("summary", "No analysis available"),
            "question": question,
            "analysis_type": "stored_summary", 
            "file_size": file_info["file_size"],
            "note": "Using stored analysis - direct Gemini analysis will be available in future updates"
        }
            
    except Exception as e:
        log.error(f"Error analyzing image: {e}")
        return {"error": f"Failed to analyze image: {str(e)}"}

def list_uploaded_files(user_id: str, chat_id: Optional[str] = None, limit: int = 20) -> Dict[str, Any]:
    """
    Tool for Gemini to list uploaded files with summaries.
    
    Args:
        user_id: The user whose files to list
        chat_id: Optional chat ID to filter files
        limit: Maximum number of files to return
        
    Returns:
        Dict with list of files and their metadata
    """
    try:
        if chat_id:
            files = get_chat_files(chat_id, user_id)
        else:
            from app.file_manager import get_user_files
            files = get_user_files(user_id, limit)
        
        # Format file list for Gemini
        file_list = []
        for file_info in files:
            file_entry = {
                "file_id": file_info["file_id"],
                "filename": file_info["original_filename"],
                "file_type": file_info["file_type"],
                "file_size": file_info["file_size"],
                "upload_date": file_info["upload_timestamp"],
                "summary": file_info.get("summary", "No summary available"),
                "processing_status": file_info["processing_status"]
            }
            
            if chat_id is None:
                file_entry["chat_id"] = file_info["chat_id"]
                
            file_list.append(file_entry)
        
        return {
            "files": file_list,
            "total_count": len(file_list),
            "chat_id": chat_id,
            "user_id": user_id
        }
        
    except Exception as e:
        log.error(f"Error listing files: {e}")
        return {"error": f"Failed to list files: {str(e)}"}

def search_user_files(user_id: str, query: str, chat_id: Optional[str] = None) -> Dict[str, Any]:
    """
    Tool for Gemini to search through user's uploaded files.
    
    Args:
        user_id: The user whose files to search
        query: Search query for filename or content
        chat_id: Optional chat ID to limit search scope
        
    Returns:
        Dict with matching files and metadata
    """
    try:
        matching_files = search_files(user_id, query, chat_id)
        
        # Format results
        file_list = []
        for file_info in matching_files:
            file_entry = {
                "file_id": file_info["file_id"],
                "filename": file_info["original_filename"],
                "file_type": file_info["file_type"],
                "summary": file_info.get("summary", "No summary available"),
                "upload_date": file_info["upload_timestamp"],
                "chat_id": file_info["chat_id"]
            }
            file_list.append(file_entry)
        
        return {
            "files": file_list,
            "search_query": query,
            "total_matches": len(file_list),
            "user_id": user_id,
            "chat_id": chat_id
        }
        
    except Exception as e:
        log.error(f"Error searching files: {e}")
        return {"error": f"Failed to search files: {str(e)}"}

def ask_question_about_files(user_id: str, question: str, chat_id: Optional[str] = None, file_ids: Optional[List[str]] = None) -> Dict[str, Any]:
    """
    Tool for Gemini to ask questions about uploaded files using native document/image understanding.
    
    Args:
        user_id: The user whose files to query
        question: The question to ask about the files
        chat_id: Optional chat ID to limit scope to specific chat
        file_ids: Optional list of specific file IDs to query
        
    Returns:
        Dict with comprehensive answers based on file content
    """
    try:
        from google import genai
        api_key = os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_API_KEY")
        
        if not api_key:
            return {"error": "Gemini API key not available for file questioning"}
        
        client = genai.Client(api_key=api_key)
        
        # Get relevant files
        if file_ids:
            # Use specific files
            files_to_query = []
            for file_id in file_ids:
                file_info = get_file_metadata(file_id, user_id)
                if file_info and file_info.get("gemini_file_id"):
                    files_to_query.append(file_info)
        else:
            # Get files from chat or all user files
            if chat_id:
                all_files = get_chat_files(chat_id, user_id)
            else:
                from app.file_manager import get_user_files
                all_files = get_user_files(user_id, 20)  # Limit for performance
            
            # Filter files that have Gemini file IDs
            files_to_query = [f for f in all_files if f.get("gemini_file_id")]
        
        if not files_to_query:
            return {"error": "No files available for questioning or files not processed with Gemini"}
        
        # Prepare content for Gemini with multiple files
        content_parts = []
        
        # Add file references
        for file_info in files_to_query:
            try:
                gemini_file = client.files.get(name=file_info["gemini_file_id"])
                content_parts.append(gemini_file)
            except Exception as e:
                log.warning(f"Could not get file {file_info['original_filename']}: {e}")
        
        if not content_parts:
            return {"error": "Could not access any files for questioning"}
        
        # Add the question
        enhanced_question = f"""Based on the uploaded files, please answer this question comprehensively:

{question}

Instructions:
1. Analyze all provided files for relevant information
2. Cite specific files when referencing information (use filenames)
3. If information spans multiple files, synthesize the information
4. If the answer is not available in the files, state that clearly
5. Focus on agricultural, farming, or land-use aspects if relevant

Question: {question}"""
        
        content_parts.append(enhanced_question)
        
        # Generate comprehensive response
        response = client.models.generate_content(
            model="gemini-2.5-flash",
            contents=content_parts
        )
        
        answer = ""
        if hasattr(response, 'text'):
            answer = response.text
        elif (hasattr(response, 'candidates') and response.candidates and 
              len(response.candidates) > 0):
            candidate = response.candidates[0]
            if (hasattr(candidate, 'content') and candidate.content and 
                hasattr(candidate.content, 'parts') and candidate.content.parts):
                answer = candidate.content.parts[0].text
        
        return {
            "question": question,
            "answer": answer,
            "files_analyzed": [
                {
                    "file_id": f.get("file_id", "unknown"),
                    "filename": f["original_filename"],
                    "file_type": f["file_type"]
                } for f in files_to_query
            ],
            "total_files": len(files_to_query),
            "user_id": user_id,
            "chat_id": chat_id,
            "analysis_type": "multi_file_gemini_qa"
        }
        
    except Exception as e:
        log.error(f"Error in file questioning: {e}")
        return {"error": f"Failed to ask question about files: {str(e)}"}
