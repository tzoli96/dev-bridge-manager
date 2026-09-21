from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.categorize_email import categorize_agent
from app.main import app

client = TestClient(app)


def test_categorize_email_returns_one_of_the_five_categories():
    with categorize_agent.override(model=TestModel(custom_output_args={"category": "szamla"})):
        response = client.post(
            "/categorize-email",
            json={
                "subject": "Számla #123 esedékes",
                "snippet": "Kérjük egyenlítsd ki a mellékelt számlát.",
                "from_address": "billing@example.com",
                "from_name": "Példa Kft.",
            },
        )
    assert response.status_code == 200
    body = response.json()
    assert body["category"] in {"ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"}
    assert body["category"] == "szamla"


def test_categorize_email_requires_all_fields():
    response = client.post("/categorize-email", json={"subject": "Hi"})
    assert response.status_code == 422
