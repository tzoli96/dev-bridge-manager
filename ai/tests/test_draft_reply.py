from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.draft_reply import draft_reply_agent
from app.main import app

client = TestClient(app)


def test_draft_reply_returns_generated_draft():
    with draft_reply_agent.override(model=TestModel(custom_output_args={"draft": "Szia! Köszönöm a megkeresésed."})):
        response = client.post(
            "/draft-reply",
            json={
                "email_content": "Szia, mikor tudnátok elkezdeni a projektet?",
                "profile_context": "Háttér: szoftverfejlesztő vállalkozó",
            },
        )
    assert response.status_code == 200
    assert response.json()["draft"] == "Szia! Köszönöm a megkeresésed."


def test_draft_reply_works_without_profile_context():
    with draft_reply_agent.override(model=TestModel(custom_output_args={"draft": "Köszönöm az emailt."})):
        response = client.post("/draft-reply", json={"email_content": "Csak egy teszt üzenet."})
    assert response.status_code == 200
    assert response.json()["draft"] == "Köszönöm az emailt."


def test_draft_reply_requires_email_content():
    response = client.post("/draft-reply", json={})
    assert response.status_code == 422
