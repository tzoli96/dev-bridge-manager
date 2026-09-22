from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.draft_reply import DraftReplyRequest, _build_prompt, draft_reply_agent
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


def test_build_prompt_includes_similar_replies_section_when_present():
    req = DraftReplyRequest(
        email_content="Szia, mikor kezdünk?",
        profile_context="",
        similar_replies=["Szia! Jövő héten kezdünk.", "Köszönöm a türelmed."],
    )
    prompt = _build_prompt(req)
    assert "korábban általad írt" in prompt
    assert "Szia! Jövő héten kezdünk." in prompt
    assert "Köszönöm a türelmed." in prompt


def test_build_prompt_omits_similar_replies_section_when_empty():
    req = DraftReplyRequest(email_content="Csak egy teszt üzenet.")
    prompt = _build_prompt(req)
    assert "korábban általad írt" not in prompt
