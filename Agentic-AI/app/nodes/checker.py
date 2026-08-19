import json, logging, os
from google import genai
from google.genai import types
from ..state import State

MODEL  = "gemini-2.5-flash"
SEARCH = types.Tool(google_search=types.GoogleSearch())
log    = logging.getLogger("Checker")

def checker_node(state: State) -> State:
    payload = {"query": state.user_query,
               "plan":  state.plan,
               "tools": state.tool_calls,
               "answer": state.draft_answer}
    prompt = (
        "Validate the answer below. "
        "Return JSON {approved:bool, issues:list, conf_delta:float}\n\n"
        + json.dumps(payload, indent=2)
    )
    # call validation model
    client = genai.Client(api_key=os.getenv('GOOGLE_API_KEY'))
    rsp = client.models.generate_content(
        model=MODEL,
        contents=prompt,
        config=types.GenerateContentConfig(tools=[SEARCH])
    )
    text = rsp.text or ""
    try:
        verdict = json.loads(text)
        state.approved   = verdict.get("approved", False)
        state.issues     = verdict.get("issues", [])
        state.confidence+= verdict.get("conf_delta", 0.0)
    except Exception:
        state.approved = True
    return state
