
import os
import sys
sys.path.append(os.getcwd())
from utils.qdrant_search import get_qdrant_client

client = get_qdrant_client()
collection_name = "news_vectors"

# Scroll some points to see payload
response = client.scroll(
    collection_name=collection_name,
    limit=5,
    with_payload=True,
    with_vectors=False
)

for point in response[0]:
    print(f"ID: {point.id}")
    print(f"Payload: {point.payload}")
    print("-" * 20)
