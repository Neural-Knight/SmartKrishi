#!/usr/bin/env python3
"""
Test the improved planner to ensure it only plans and doesn't execute tools
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.state import State
from app.nodes.planner import planner_node

def test_planner_scenarios():
    """Test various scenarios to ensure planner only creates plans"""
    
    print("🧠 Testing Improved Planner")
    print("=" * 50)
    
    scenarios = [
        {
            "query": "What's the weather like in Iowa for corn planting?",
            "expected_tools": ["weather_api"],
            "expected_intent": "weather_forecast"
        },
        {
            "query": "I uploaded an image of my corn crop, can you analyze it for diseases?",
            "expected_tools": ["get_image_analysis"],
            "expected_intent": "crop_disease_analysis"
        },
        {
            "query": "What are the current market prices for soybeans?",
            "expected_tools": ["market_api"],
            "expected_intent": "market_analysis"
        },
        {
            "query": "My soil seems unhealthy, what should I do? Also check the weather.",
            "expected_tools": ["soil_api", "weather_api"],
            "expected_intent": "soil_health_advice"
        },
        {
            "query": "I have a PDF with farming guidelines, can you help me understand it?",
            "expected_tools": ["ask_question_about_files"],
            "expected_intent": "file_analysis"
        }
    ]
    
    for i, scenario in enumerate(scenarios, 1):
        print(f"\n📝 Scenario {i}: {scenario['query']}")
        print("-" * 30)
        
        # Create state
        state = State(
            user_id="test_user",
            chat_id="test_chat", 
            user_query=scenario['query']
        )
        
        # Run planner
        try:
            result_state = planner_node(state)
            plan = result_state.plan
            
            print(f"✅ Plan created: {plan}")
            
            # Check if plan looks reasonable
            if 'tools_needed' in plan:
                tools = plan['tools_needed']
                print(f"🔧 Tools planned: {tools}")
                
                # Check if any expected tools are included
                has_expected = any(tool in tools for tool in scenario['expected_tools'])
                if has_expected:
                    print("✅ Contains expected tools")
                else:
                    print(f"⚠️  Expected tools {scenario['expected_tools']} not found")
            
            if 'reasoning' in plan:
                print(f"💭 Reasoning: {plan['reasoning']}")
                
        except Exception as e:
            print(f"❌ Error: {e}")
            import traceback
            traceback.print_exc()

if __name__ == "__main__":
    test_planner_scenarios()
