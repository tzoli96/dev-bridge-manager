from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.main import app
from app.match_email_client import match_email_client_agent

client = TestClient(app)

CLIENTS = [
    {"id": 1, "name": "Acme Kft.", "email": "info@acme.hu"},
    {"id": 2, "name": "Beta Ltd.", "email": "hello@beta.com"},
]


def test_match_email_client_returns_matched_id():
    with match_email_client_agent.override(model=TestModel(custom_output_args={"client_id": 2})):
        response = client.post(
            "/match-email-client",
            json={
                "subject": "Project update",
                "snippet": "Here's the update you asked for.",
                "from_address": "jane@beta.com",
                "from_name": "Jane at Beta",
                "clients": CLIENTS,
            },
        )
    assert response.status_code == 200
    assert response.json()["client_id"] == 2


def test_match_email_client_returns_null_when_agent_says_no_match():
    with match_email_client_agent.override(model=TestModel(custom_output_args={"client_id": None})):
        response = client.post(
            "/match-email-client",
            json={
                "subject": "Newsletter",
                "snippet": "...",
                "from_address": "someone@unrelated.com",
                "from_name": "Someone",
                "clients": CLIENTS,
            },
        )
    assert response.status_code == 200
    assert response.json()["client_id"] is None


def test_match_email_client_rejects_id_not_in_candidate_list():
    with match_email_client_agent.override(model=TestModel(custom_output_args={"client_id": 999})):
        response = client.post(
            "/match-email-client",
            json={
                "subject": "s",
                "snippet": "s",
                "from_address": "a@b.com",
                "from_name": "n",
                "clients": CLIENTS,
            },
        )
    assert response.status_code == 200
    assert response.json()["client_id"] is None


def test_match_email_client_returns_null_with_no_candidates():
    response = client.post(
        "/match-email-client",
        json={
            "subject": "s",
            "snippet": "s",
            "from_address": "a@b.com",
            "from_name": "n",
            "clients": [],
        },
    )
    assert response.status_code == 200
    assert response.json()["client_id"] is None


def test_match_email_client_requires_all_fields():
    response = client.post("/match-email-client", json={"subject": "Hi"})
    assert response.status_code == 422
