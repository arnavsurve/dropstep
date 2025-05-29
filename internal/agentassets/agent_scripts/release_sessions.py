from steel import Steel
import os
from dotenv import load_dotenv

load_dotenv()
client = Steel(steel_api_key=os.getenv("STEEL_API_KEY"))

# This will lazily iterate through all sessions
session_count = 0
for session in client.sessions.list():
    print(f"Releasing session: {session.id}")
    client.sessions.release(session.id)
    session_count += 1

print(f"Released {session_count} session(s).")
