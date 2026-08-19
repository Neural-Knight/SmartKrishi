import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api

client = TestClient(api)

print("1) GET /tools")
print(json.dumps(client.get("/tools").json(), indent=2))
print()

print("2) GET /users/alice/tools")
print(json.dumps(client.get("/users/alice/tools").json(), indent=2))
print()

print("3) POST /users/alice/tools -> ['weather_api','soil_api']")
print(json.dumps(
    client.post("/users/alice/tools", json=["weather_api","soil_api"]).json(),
    indent=2
))
print()

print("4) POST /ask -> 'What\u2019s the weather in Paris?' (non-stream)")
r1 = client.post(
    "/ask",
    data={"q": "What’s the weather in Paris?", "user_id": "alice"}
)
print(json.dumps(r1.json(), indent=2))
print()

print("5) /ask_stream -> 'What’s the weather in Paris?' (stream)")
with client.stream(
    "POST", "/ask_stream",
    data={"q": "What’s the weather in Paris?", "user_id": "alice", "include_tools": "weather_api"}
) as r2:
    for line in r2.iter_lines():
        if line:
            print(line)
