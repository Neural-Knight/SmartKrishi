import os
import uuid
import datetime as dt
import logging
import time
from typing import Literal, Optional, Dict, Any, List
from pypdf import PdfReader
import chromadb
from google import genai
from PIL import Image
from docx2pdf import convert
import pandas as pd
from app.file_manager import (
    store_file_metadata, update_file_processing, 
    get_file_metadata, get_chat_files, get_user_files
)
from app.client_manager import client_manager

log = logging.getLogger("Media")

UPLOAD_ROOT = "uploads"
VECTOR_DIR = "data/vectors"
os.makedirs(UPLOAD_ROOT, exist_ok=True)
os.makedirs(VECTOR_DIR, exist_ok=True)

# Configure Gemini
api_key = os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_API_KEY")
if api_key:
    try:
        # Test client creation
        test_client = genai.Client(api_key=api_key)
        log.info("Gemini client configured successfully")
    except Exception as e:
        log.warning(f"Gemini client configuration failed: {e}")
        api_key = None
else:
    log.warning("GEMINI_API_KEY/GOOGLE_API_KEY not found - file processing will be limited")

client = chromadb.PersistentClient(path=VECTOR_DIR)
PDF_COL = client.get_or_create_collection("pdfs")

def _uid_name(fname: str) -> str:
    stem, ext = os.path.splitext(fname)
    return f"{stem}_{dt.datetime.utcnow():%Y%m%d_%H%M%S}_{uuid.uuid4().hex[:6]}{ext}"

def save_file(user: str, content: bytes, filename: str,
              kind: Literal["pdf", "image", "docx", "xlsx", "csv"], chat_id: str) -> Dict[str, Any]:
    """Enhanced file saving with Gemini integration and metadata tracking"""
    
    # Generate unique file ID and path
    file_id = str(uuid.uuid4())
    folder = os.path.join(UPLOAD_ROOT, user)
    os.makedirs(folder, exist_ok=True)
    stored_filename = _uid_name(filename)
    stored_path = os.path.join(folder, stored_filename)
    
    # Save file to disk
    with open(stored_path, "wb") as f:
        f.write(content)
    
    file_size = len(content)
    log.info(f"File saved: {stored_path} ({file_size} bytes)")
    
    # Store initial metadata
    store_file_metadata(
        file_id=file_id,
        chat_id=chat_id,
        user_id=user,
        original_filename=filename,
        stored_path=stored_path,
        file_type=kind,
        file_size=file_size,
        processing_status="processing"
    )
    
    # Process file with Gemini in background
    try:
        if kind == "pdf":
            result = _process_pdf_with_gemini(file_id, user, stored_path, filename)
        elif kind == "image":
            result = _process_image_with_gemini(file_id, stored_path, filename, chat_id)
        elif kind == "docx":
            result = _process_docx_with_gemini(file_id, user, stored_path, filename)
        elif kind == "xlsx":
            result = _process_xlsx_with_gemini(file_id, user, stored_path, filename)
        elif kind == "csv":
            result = _process_csv_with_gemini(file_id, user, stored_path, filename, chat_id)
        else:
            result = {"success": False, "error": "Unsupported file type"}
        
        if result.get("success"):
            # Special handling for gemini_file_id if it exists separately from gemini_file_ref
            gemini_file_ref = result.get("gemini_file_ref")
            if result.get("gemini_file_id") and not gemini_file_ref:
                # For cases where we have file_id but not the ref object, create a mock ref
                class MockFileRef:
                    def __init__(self, file_id):
                        self.name = file_id
                gemini_file_ref = MockFileRef(result.get("gemini_file_id"))
            
            update_file_processing(
                file_id=file_id,
                gemini_file_ref=gemini_file_ref,
                summary=result.get("summary"),
                content_preview=result.get("content_preview"),
                processing_status="completed"
            )
        else:
            update_file_processing(
                file_id=file_id,
                processing_status="failed"
            )
            
    except Exception as e:
        log.error(f"File processing failed for {file_id}: {e}")
        update_file_processing(file_id=file_id, processing_status="failed")
    
    return {
        "file_id": file_id,
        "stored_path": stored_path,
        "processing_status": "processing"
    }

def _process_pdf_with_gemini(file_id: str, user: str, path: str, filename: str) -> Dict[str, Any]:
    """Process PDF with Gemini File API for comprehensive document understanding"""
    try:
        # Extract text for ChromaDB (for search/retrieval)
        text = "\n".join(page.extract_text() or "" for page in PdfReader(path).pages)
        
        # Add to ChromaDB for text search capabilities
        PDF_COL.add(
            ids=[f"{user}:{file_id}"],
            documents=[text],
            metadatas=[{"user": user, "file_id": file_id, "filename": filename, "path": path}]
        )
        
        # Upload to Gemini File API for native document processing
        gemini_file_ref = None
        summary = None
        
        if api_key:
            try:
                client = genai.Client(api_key=api_key)
                
                # Upload PDF using File API for native document understanding
                upload_file = client.files.upload(file=path)
                gemini_file_ref = upload_file  # Store the file reference
                
                # Generate comprehensive summary using Gemini's document understanding
                summary_response = client.models.generate_content(
                    model="gemini-2.5-flash",
                    contents=[
                        upload_file,
                        """Analyze this document and provide:
1. A concise 2-3 sentence summary
2. Key topics and main points
3. Important data, findings, or conclusions
4. Relevance to agriculture (if applicable)

Format as a comprehensive summary."""
                    ]
                )
                
                if hasattr(summary_response, 'text'):
                    summary = summary_response.text
                elif (hasattr(summary_response, 'candidates') and summary_response.candidates and 
                      len(summary_response.candidates) > 0):
                    candidate = summary_response.candidates[0]
                    if (hasattr(candidate, 'content') and candidate.content and 
                        hasattr(candidate.content, 'parts') and candidate.content.parts):
                        summary = candidate.content.parts[0].text
                
                log.info(f"PDF processed with Gemini File API: {filename}")
                
            except Exception as e:
                log.error(f"Gemini File API processing failed for {filename}: {e}")
                # Still create a basic summary from text
                summary = f"PDF document '{filename}' containing {len(text)} characters of agricultural content."
        
        # Create content preview and fallback summary
        content_preview = text[:500] + "..." if len(text) > 500 else text
        if not summary:
            summary = f"PDF document '{filename}' with {len(text)} characters of text content."
        
        return {
            "success": True,
            "gemini_file_ref": gemini_file_ref,  # Store the actual file reference
            "summary": summary,
            "content_preview": content_preview
        }
        
    except Exception as e:
        log.error(f"PDF processing failed: {e}")
        return {"success": False, "error": str(e)}

def _process_image_with_gemini(file_id: str, path: str, filename: str, chat_id: str) -> Dict[str, Any]:
    """Process image with Gemini Vision API for comprehensive image understanding"""
    try:
        summary = f"Image file '{filename}'"
        content_preview = "Image content available for analysis"
        gemini_file_ref = None
        gemini_file_id = None
        
        if api_key:
            try:
                # Use client manager to upload image - this ensures consistency for conversations
                upload_file = client_manager.upload_file(
                    chat_id=chat_id,
                    file_path=path,
                    display_name=filename,
                    mime_type="image/jpeg" if filename.lower().endswith(('.jpg', '.jpeg')) else "image/png"
                )
                
                if upload_file:
                    gemini_file_ref = upload_file
                    gemini_file_id = upload_file.name  # Store the Gemini file ID for conversation access
                    
                    # Generate comprehensive image analysis using client manager
                    analysis_response = client_manager.generate_content(
                        chat_id=chat_id,
                        model="gemini-2.5-flash",
                        contents=[
                            upload_file,
                            """Analyze this image comprehensively for agricultural purposes:
1. Describe what you see in detail (crops, plants, equipment, land, buildings, etc.)
2. Identify any agricultural elements (crop types, growth stages, farming equipment, livestock, etc.)
3. Note any issues or concerns (pests, diseases, equipment problems, soil conditions)
4. Assess any visible data, charts, or technical information
5. Provide relevant agricultural insights and recommendations

Provide a detailed analysis focused on agricultural context and actionable insights."""
                        ]
                    )
                    
                    if analysis_response and hasattr(analysis_response, 'text'):
                        summary = analysis_response.text
                        content_preview = f"Agricultural image analysis: {analysis_response.text[:200]}..."
                    elif (analysis_response and hasattr(analysis_response, 'candidates') and 
                          analysis_response.candidates and len(analysis_response.candidates) > 0):
                        candidate = analysis_response.candidates[0]
                        if (hasattr(candidate, 'content') and candidate.content and 
                            hasattr(candidate.content, 'parts') and candidate.content.parts):
                            summary = candidate.content.parts[0].text
                            content_preview = f"Agricultural image analysis: {summary[:200]}..."
                    
                    log.info(f"Image processed with Gemini Vision API via client manager: {filename}")
                else:
                    log.error(f"Failed to upload image to Gemini via client manager: {filename}")
                    
            except Exception as e:
                log.error(f"Gemini Vision API processing failed for {filename}: {e}")
        
        # Fallback to basic image info if Gemini processing fails
        if not gemini_file_ref:
            try:
                from PIL import Image
                with Image.open(path) as img:
                    width, height = img.size
                    mode = img.mode
                    summary = f"Image '{filename}' - {width}x{height} pixels, {mode} mode"
                    content_preview = f"Image dimensions: {width}x{height} {mode}"
            except Exception:
                pass
        
        return {
            "success": True,
            "gemini_file_ref": gemini_file_ref,  # Store the actual file reference
            "gemini_file_id": gemini_file_id,    # Store the file ID for conversation access
            "summary": summary,
            "content_preview": content_preview
        }
        
    except Exception as e:
        log.error(f"Image processing failed: {e}")
        return {"success": False, "error": str(e)}

def _process_docx_with_gemini(file_id: str, user: str, path: str, filename: str) -> Dict[str, Any]:
    """Process DOCX files by converting to PDF first, then using Gemini's PDF processing capabilities"""
    try:
        gemini_file_ref = None
        summary = None
        content_preview = None
        
        # Convert DOCX to PDF first
        pdf_path = path.replace('.docx', '.pdf').replace('.DOCX', '.pdf')
        try:
            convert(path, pdf_path)
            log.info(f"Successfully converted DOCX to PDF: {filename} -> {os.path.basename(pdf_path)}")
        except Exception as e:
            log.error(f"DOCX to PDF conversion failed for {filename}: {e}")
            # Try fallback with python-docx for text extraction
            try:
                from docx import Document
                doc = Document(path)
                text_content = []
                for paragraph in doc.paragraphs:
                    if paragraph.text.strip():
                        text_content.append(paragraph.text)
                
                # Create a simple text summary
                summary = f"DOCX Document '{filename}' - {len(text_content)} paragraphs"
                content_preview = " ".join(text_content[:3]) if text_content else "No content found"
                
                return {
                    "success": True,
                    "gemini_file_ref": None,
                    "summary": summary,
                    "content_preview": content_preview
                }
            except Exception as docx_error:
                log.error(f"Fallback DOCX text extraction failed: {docx_error}")
                return {"success": False, "error": f"DOCX processing failed: {str(e)}"}
        
        # Now process the converted PDF with Gemini
        if api_key and os.path.exists(pdf_path):
            try:
                client = genai.Client(api_key=api_key)
                
                # Upload PDF using File API for native document processing
                upload_file = client.files.upload(file=pdf_path)
                gemini_file_ref = upload_file  # Store the file reference
                
                # Generate comprehensive summary using Gemini's document understanding
                summary_response = client.models.generate_content(
                    model="gemini-2.5-flash",
                    contents=[
                        upload_file,
                        f"""Analyze this PDF document (originally a DOCX file: {filename}) and provide:
1. A concise 2-3 sentence summary
2. Key topics and main points
3. Important data, findings, or conclusions
4. Relevance to agriculture (if applicable)
5. Document structure and content overview

Format as a comprehensive summary. Note: This document was converted from DOCX to PDF format for processing."""
                    ]
                )
                
                if hasattr(summary_response, 'text'):
                    summary = summary_response.text
                elif (hasattr(summary_response, 'candidates') and summary_response.candidates and 
                      len(summary_response.candidates) > 0):
                    candidate = summary_response.candidates[0]
                    if (hasattr(candidate, 'content') and candidate.content and 
                        hasattr(candidate.content, 'parts') and candidate.content.parts):
                        summary = candidate.content.parts[0].text
                
                # Extract text for content preview
                try:
                    text = "\n".join(page.extract_text() or "" for page in PdfReader(pdf_path).pages)
                    content_preview = text[:500] + "..." if len(text) > 500 else text
                except Exception:
                    content_preview = "PDF content converted from DOCX"
                
                log.info(f"DOCX converted to PDF and processed with Gemini: {filename}")
                
                # Clean up temporary PDF file
                try:
                    os.remove(pdf_path)
                    log.info(f"Cleaned up temporary PDF file: {pdf_path}")
                except Exception as cleanup_error:
                    log.warning(f"Could not clean up temporary PDF file {pdf_path}: {cleanup_error}")
                
            except Exception as e:
                log.error(f"Gemini PDF processing failed for converted DOCX {filename}: {e}")
                # Clean up PDF file on error too
                try:
                    if os.path.exists(pdf_path):
                        os.remove(pdf_path)
                except Exception:
                    pass
                
                # Fallback to text extraction from original DOCX
                try:
                    from docx import Document
                    doc = Document(path)
                    text_content = []
                    for paragraph in doc.paragraphs:
                        if paragraph.text.strip():
                            text_content.append(paragraph.text)
                    summary = f"DOCX Document '{filename}' (conversion failed, processed as text) - {len(text_content)} paragraphs"
                    content_preview = " ".join(text_content[:3]) if text_content else "No content found"
                except Exception:
                    summary = f"DOCX file: {filename} (processing failed)"
                    content_preview = "Could not extract content"
        else:
            # Fallback when no API key or PDF conversion failed
            try:
                from docx import Document
                doc = Document(path)
                text_content = []
                for paragraph in doc.paragraphs:
                    if paragraph.text.strip():
                        text_content.append(paragraph.text)
                summary = f"DOCX Document '{filename}' - {len(text_content)} paragraphs"
                content_preview = " ".join(text_content[:3]) if text_content else "No content found"
            except Exception:
                summary = f"DOCX file: {filename}"
                content_preview = "Could not extract content"
        
        return {
            "success": True,
            "gemini_file_ref": gemini_file_ref,
            "summary": summary,
            "content_preview": content_preview
        }
        
    except Exception as e:
        log.error(f"DOCX processing failed: {e}")
        return {"success": False, "error": str(e)}

def _process_xlsx_with_gemini(file_id: str, user: str, path: str, filename: str) -> Dict[str, Any]:
    """Process XLSX files by converting to CSV first, then using Gemini's CSV processing capabilities"""
    try:
        gemini_file_ref = None
        summary = None
        content_preview = None
        
        # Convert XLSX to CSV first
        csv_path = path.replace('.xlsx', '.csv').replace('.XLSX', '.csv')
        try:
            # Read the XLSX file and convert to CSV
            # If there are multiple sheets, we'll process the first sheet or combine them
            excel_file = pd.ExcelFile(path)
            sheet_names = excel_file.sheet_names
            
            if len(sheet_names) == 1:
                # Single sheet - convert directly
                df = pd.read_excel(path, sheet_name=sheet_names[0])
                df.to_csv(csv_path, index=False)
                conversion_note = f"Converted single sheet '{sheet_names[0]}' to CSV"
            else:
                # Multiple sheets - combine or process the first sheet
                # For now, let's process the first sheet and note others exist
                df = pd.read_excel(path, sheet_name=sheet_names[0])
                df.to_csv(csv_path, index=False)
                conversion_note = f"Converted first sheet '{sheet_names[0]}' to CSV (file has {len(sheet_names)} sheets: {', '.join(str(name) for name in sheet_names)})"
            
            log.info(f"Successfully converted XLSX to CSV: {filename} -> {os.path.basename(csv_path)}")
            log.info(conversion_note)
            
        except Exception as e:
            log.error(f"XLSX to CSV conversion failed for {filename}: {e}")
            # Try fallback with openpyxl for basic info
            try:
                import openpyxl
                workbook = openpyxl.load_workbook(path, read_only=True)
                sheet_names = workbook.sheetnames
                summary = f"Excel file '{filename}' - {len(sheet_names)} sheets: {', '.join(sheet_names)} (conversion to CSV failed)"
                content_preview = f"Sheets: {', '.join(sheet_names[:3])}"
                workbook.close()
                
                return {
                    "success": True,
                    "gemini_file_ref": None,
                    "summary": summary,
                    "content_preview": content_preview
                }
            except Exception as xlsx_error:
                log.error(f"Fallback XLSX info extraction failed: {xlsx_error}")
                return {"success": False, "error": f"XLSX processing failed: {str(e)}"}
        
        # Now process the converted CSV with the existing CSV processing logic
        if api_key and os.path.exists(csv_path):
            try:
                # Use the client manager to ensure consistency (similar to CSV processing)
                uploaded_file = client_manager.upload_file(
                    file_id,  # Using file_id as chat_id for now
                    csv_path, 
                    display_name=f"{filename} (converted to CSV)",
                    mime_type="text/csv"
                )
                
                if uploaded_file:
                    gemini_file_ref = uploaded_file
                    
                    # Create proper Part object with fileData for code execution
                    file_part = genai.types.Part(file_data=genai.types.FileData(file_uri=uploaded_file.uri))
                    
                    # Use code execution for comprehensive data analysis
                    analysis_response = client_manager.generate_content(
                        chat_id=file_id,  # Using file_id as chat_id
                        model="gemini-2.5-flash",
                        contents=[
                            file_part,
                            f"""Analyze this CSV file (originally an Excel file: {filename}) using code execution:

IMPORTANT: The CSV file has been uploaded and is available in your environment. 
This data was converted from an Excel spreadsheet.

Steps to perform:
1. Load the CSV data (the file is already available in your environment)
2. Examine the data structure, columns, and data types
3. Analyze data patterns, trends, and key statistics  
4. Look for agricultural data (crop yields, weather data, market prices, farm operations, etc.)
5. Create summary statistics and insights
6. Generate visualizations if appropriate for the data
7. Detect missing values, outliers, and data issues
8. Provide recommendations based on the data

Note: This data was originally from an Excel file with the following conversion info: {conversion_note}
Use df = pd.read_csv() or access the uploaded file data directly. Do not reference local file paths."""
                        ],
                        config=genai.types.GenerateContentConfig()
                    )
                    
                    if analysis_response and hasattr(analysis_response, 'text') and analysis_response.text:
                        summary = analysis_response.text
                        content_preview = f"XLSX→CSV analysis: {analysis_response.text[:500]}..."
                    elif (analysis_response and hasattr(analysis_response, 'candidates') and analysis_response.candidates and
                          len(analysis_response.candidates) > 0):
                        candidate = analysis_response.candidates[0]
                        if (hasattr(candidate, 'content') and candidate.content and 
                            hasattr(candidate.content, 'parts') and candidate.content.parts):
                            text_parts = [part.text for part in candidate.content.parts if hasattr(part, 'text') and part.text]
                            if text_parts:
                                summary = "\n".join(text_parts)
                                content_preview = f"XLSX→CSV analysis: {summary[:500]}..."
                            else:
                                summary = f"Excel file '{filename}' converted to CSV and processed successfully with code execution but no text output."
                                content_preview = f"XLSX→CSV analysis completed. {conversion_note}"
                        else:
                            summary = f"Excel file '{filename}' converted to CSV and processed successfully."
                            content_preview = f"XLSX→CSV analysis completed. {conversion_note}"
                    else:
                        summary = f"Excel file '{filename}' converted to CSV and processed successfully."
                        content_preview = f"XLSX→CSV analysis completed. {conversion_note}"
                        
                    log.info(f"XLSX converted to CSV and analyzed successfully with Gemini: {filename}")
                else:
                    # Fallback: basic file info with conversion note
                    summary = f"Excel file '{filename}' converted to CSV successfully. Analysis with Gemini failed."
                    content_preview = f"XLSX file converted to CSV. {conversion_note}"
                
                # Clean up temporary CSV file
                try:
                    os.remove(csv_path)
                    log.info(f"Cleaned up temporary CSV file: {csv_path}")
                except Exception as cleanup_error:
                    log.warning(f"Could not clean up temporary CSV file {csv_path}: {cleanup_error}")
                
            except Exception as e:
                log.error(f"Gemini CSV processing failed for converted XLSX {filename}: {e}")
                # Clean up CSV file on error too
                try:
                    if os.path.exists(csv_path):
                        os.remove(csv_path)
                except Exception:
                    pass
                
                # Fallback to basic XLSX info
                try:
                    import openpyxl
                    workbook = openpyxl.load_workbook(path, read_only=True)
                    sheet_names = workbook.sheetnames
                    summary = f"Excel file '{filename}' (conversion successful, but Gemini processing failed) - {len(sheet_names)} sheets: {', '.join(sheet_names)}"
                    content_preview = f"Sheets: {', '.join(sheet_names[:3])}"
                    workbook.close()
                except Exception:
                    summary = f"XLSX file: {filename} (processing failed)"
                    content_preview = "Could not extract content"
        else:
            # Fallback when no API key or CSV conversion failed
            try:
                import openpyxl
                workbook = openpyxl.load_workbook(path, read_only=True)
                sheet_names = workbook.sheetnames
                summary = f"Excel file '{filename}' - {len(sheet_names)} sheets: {', '.join(sheet_names)}"
                content_preview = f"Sheets: {', '.join(sheet_names[:3])}"
                workbook.close()
            except Exception:
                summary = f"XLSX file: {filename}"
                content_preview = "Could not extract content"
        
        return {
            "success": True,
            "gemini_file_ref": gemini_file_ref,
            "summary": summary,
            "content_preview": content_preview
        }
        
    except Exception as e:
        log.error(f"XLSX processing failed: {e}")
        return {"success": False, "error": str(e)}

def _process_csv_with_gemini(file_id: str, user: str, path: str, filename: str, chat_id: str) -> Dict[str, Any]:
    """Process CSV files with Gemini's code execution for data analysis and visualization"""
    try:
        gemini_file_ref = None
        summary = None
        content_preview = None
        
        # Use the client manager to ensure consistency
        uploaded_file = client_manager.upload_file(
            chat_id, 
            path, 
            display_name=filename,
            mime_type="text/csv"
        )
        
        if uploaded_file:
            gemini_file_ref = uploaded_file
            
            # Create proper Part object with fileData for code execution
            file_part = genai.types.Part(file_data=genai.types.FileData(file_uri=uploaded_file.uri))
            
            # Use code execution for comprehensive data analysis
            analysis_response = client_manager.generate_content(
                chat_id=chat_id,
                model="gemini-2.5-flash",
                contents=[
                    file_part,  # Use the proper Part object
                    """Analyze this CSV file using code execution:

IMPORTANT: The CSV file has been uploaded and is available in your environment. 
You can access it directly using pandas without specifying a file path.

Steps to perform:
1. Load the CSV data (the file is already available in your environment)
2. Examine the data structure, columns, and data types
3. Analyze data patterns, trends, and key statistics  
4. Look for agricultural data (crop yields, weather data, market prices, farm operations, etc.)
5. Create summary statistics and insights
6. Generate visualizations if appropriate for the data
7. Detect missing values, outliers, and data issues
8. Provide recommendations based on the data

Note: Use df = pd.read_csv() or access the uploaded file data directly. Do not reference local file paths."""
                ],
                config=genai.types.GenerateContentConfig()
            )
            
            if analysis_response and hasattr(analysis_response, 'text') and analysis_response.text:
                summary = analysis_response.text
                content_preview = f"CSV analysis: {analysis_response.text[:500]}..."
            elif (analysis_response and hasattr(analysis_response, 'candidates') and analysis_response.candidates and
                  len(analysis_response.candidates) > 0):
                candidate = analysis_response.candidates[0]
                if (hasattr(candidate, 'content') and candidate.content and 
                    hasattr(candidate.content, 'parts') and candidate.content.parts):
                    text_parts = [part.text for part in candidate.content.parts if hasattr(part, 'text') and part.text]
                    if text_parts:
                        summary = "\n".join(text_parts)
                        content_preview = f"CSV analysis: {summary[:500]}..."
                    else:
                        summary = "CSV file processed successfully with code execution but no text output."
                        content_preview = "CSV analysis completed."
                else:
                    summary = "CSV file processed successfully."
                    content_preview = "CSV analysis completed."
            else:
                summary = "CSV file processed successfully."
                content_preview = "CSV analysis completed."
                
            log.info(f"CSV {filename} analyzed successfully with Gemini")
        else:
            # Fallback: basic file info
            summary = f"CSV file '{filename}' uploaded successfully. Analysis with Gemini failed."
            content_preview = f"CSV file: {filename}"
        
        return {
            "success": True,
            "gemini_file_ref": gemini_file_ref,
            "summary": summary,
            "content_preview": content_preview,
            "file_type": "csv"
        }
        
    except Exception as e:
        log.error(f"CSV processing failed: {e}")
        return {"success": False, "error": str(e)}

def _embed_pdf(user: str, path: str):
    """Legacy function - kept for backward compatibility"""
    try:
        text = "\n".join(page.extract_text() or "" for page in PdfReader(path).pages)
        PDF_COL.add(ids=[f"{user}:{path}"],
                    documents=[text],
                    metadatas=[{"user": user, "file": path}])
        log.info("PDF embedded: %s", path)
    except Exception as e:
        log.error("PDF ingest failed: %s", e)
