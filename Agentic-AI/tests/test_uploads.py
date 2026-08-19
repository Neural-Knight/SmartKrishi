import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from fastapi.testclient import TestClient
import json
from app.main import api
import io

client = TestClient(api)

print("Testing Upload Endpoints")
print("=" * 50)

# Test PDF upload
print("1) Testing PDF upload")
pdf_content = b"%PDF-1.4\n%Test PDF content\n%%EOF"
files = {"file": ("test.pdf", io.BytesIO(pdf_content), "application/pdf")}
data = {"user_id": "test_user"}

response = client.post("/upload/pdf", files=files, data=data)
print(f"Status: {response.status_code}")
print(f"Response: {json.dumps(response.json(), indent=2)}")
print()

# Test image upload
print("2) Testing Image upload")
image_content = b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xdb\x00\x00\x00\x00IEND\xaeB`\x82"
files = {"file": ("test.png", io.BytesIO(image_content), "image/png")}
data = {"user_id": "test_user"}

response = client.post("/upload/image", files=files, data=data)
print(f"Status: {response.status_code}")
print(f"Response: {json.dumps(response.json(), indent=2)}")
print()

# Test upload without filename
print("3) Testing upload without filename")
files = {"file": io.BytesIO(pdf_content)}
data = {"user_id": "test_user"}

response = client.post("/upload/pdf", files=files, data=data)
print(f"Status: {response.status_code}")
print(f"Response: {json.dumps(response.json(), indent=2)}")
