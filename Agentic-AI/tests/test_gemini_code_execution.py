#!/usr/bin/env python3
"""
Test Gemini Code Execution
This script tests if Gemini's code execution feature works and how to capture the events.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import google.genai as genai
from google.genai import types

# Set up API key
os.environ['GOOGLE_API_KEY'] = os.getenv('GOOGLE_API_KEY', 'your-api-key-here')

def test_code_execution():
    """Test Gemini code execution with a simple math problem"""
    
    client = genai.Client()
    
    # Create a prompt that should trigger code execution
    prompt = """
    I have a dataset of corn yield data for the last 5 years:
    Year 1: 150 bushels/acre
    Year 2: 165 bushels/acre  
    Year 3: 142 bushels/acre
    Year 4: 178 bushels/acre
    Year 5: 156 bushels/acre
    
    Please calculate the average yield, standard deviation, and create a simple visualization showing the trend. Use Python code to do the calculations and create a plot.
    """
    
    config = types.GenerateContentConfig(
        tools=[
            types.Tool(code_execution=types.ToolCodeExecution())
        ]
    )
    
    print("🌾 Testing Gemini Code Execution...")
    print("=" * 50)
    print(f"Prompt: {prompt}")
    print("\n" + "=" * 50)
    print("Response:")
    
    try:
        # Use streaming to see all events
        for chunk in client.models.generate_content_stream(
            model="gemini-2.0-flash-exp",  # Using 2.0 flash for code execution
            contents=prompt,
            config=config
        ):
            print(f"Chunk type: {type(chunk)}")
            print(f"Chunk: {chunk}")
            
            # Check for candidates
            if hasattr(chunk, 'candidates') and chunk.candidates:
                for candidate in chunk.candidates:
                    print(f"Candidate: {candidate}")
                    
                    # Check for content
                    if hasattr(candidate, 'content') and candidate.content:
                        print(f"Content: {candidate.content}")
                        
                        # Check for parts
                        if hasattr(candidate.content, 'parts') and candidate.content.parts:
                            for part in candidate.content.parts:
                                print(f"Part type: {type(part)}")
                                print(f"Part: {part}")
                                
                                # Check if it's a text part
                                if hasattr(part, 'text') and part.text:
                                    print(f"Text: {part.text}")
                                    
                                # Check if it's an executable code part
                                if hasattr(part, 'executable_code'):
                                    print(f"🔥 EXECUTABLE CODE FOUND!")
                                    print(f"Language: {getattr(part.executable_code, 'language', 'unknown')}")
                                    print(f"Code: {getattr(part.executable_code, 'code', 'no code')}")
                                    
                                # Check if it's a code execution result
                                if hasattr(part, 'code_execution_result'):
                                    print(f"🎯 CODE EXECUTION RESULT FOUND!")
                                    print(f"Outcome: {getattr(part.code_execution_result, 'outcome', 'unknown')}")
                                    print(f"Output: {getattr(part.code_execution_result, 'output', 'no output')}")
            
            print("-" * 30)
            
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    if not os.getenv('GOOGLE_API_KEY'):
        print("❌ Please set GOOGLE_API_KEY environment variable")
        sys.exit(1)
    
    test_code_execution()
