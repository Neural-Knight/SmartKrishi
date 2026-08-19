from dataclasses import dataclass, field
from typing import Dict, Any, List

@dataclass
class State:
    user_id: str                       # ← multi-user support
    chat_id: str                       # ← chat support
    user_query: str
    plan: Dict[str, Any]              = field(default_factory=dict)
    history: List[Dict[str, str]]     = field(default_factory=list)
    tool_calls: Dict[str, Any]        = field(default_factory=dict)
    draft_answer: str                 = ""
    approved: bool                    = False
    issues:   List[str]               = field(default_factory=list)
    confidence: float                 = 0.7
