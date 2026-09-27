from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.client_status_email import ClientStatusEmailRequest, _build_prompt, client_status_email_agent
from app.main import app

client = TestClient(app)


def test_client_status_email_returns_generated_subject_and_body():
    with client_status_email_agent.override(
        model=TestModel(custom_output_args={"subject": "Heti státusz", "body": "Szia! Ezen a héten..."})
    ):
        response = client.post(
            "/client-status-email",
            json={
                "client_name": "Acme Kft.",
                "period_start": "2026-09-21",
                "period_end": "2026-09-27",
                "completed_tasks": ["Bejelentkezés oldal fejlesztése"],
                "hours_logged": 12.5,
                "invoices_created": 1,
                "invoices_created_total": 250000.0,
                "invoices_paid": 0,
            },
        )
    assert response.status_code == 200
    body = response.json()
    assert body["subject"] == "Heti státusz"
    assert body["body"] == "Szia! Ezen a héten..."


def test_client_status_email_works_with_only_required_fields():
    with client_status_email_agent.override(
        model=TestModel(custom_output_args={"subject": "s", "body": "b"})
    ):
        response = client.post(
            "/client-status-email",
            json={"client_name": "Acme Kft.", "period_start": "2026-09-21", "period_end": "2026-09-27"},
        )
    assert response.status_code == 200


def test_build_prompt_includes_completed_tasks_when_present():
    req = ClientStatusEmailRequest(
        client_name="Acme Kft.",
        period_start="2026-09-21",
        period_end="2026-09-27",
        completed_tasks=["Feladat A", "Feladat B"],
    )
    prompt = _build_prompt(req)
    assert "Feladat A" in prompt
    assert "Feladat B" in prompt


def test_build_prompt_omits_completed_tasks_section_when_empty():
    req = ClientStatusEmailRequest(client_name="Acme Kft.", period_start="2026-09-21", period_end="2026-09-27")
    prompt = _build_prompt(req)
    assert "elkészült feladatok" not in prompt


def test_build_prompt_includes_hours_logged_when_present():
    req = ClientStatusEmailRequest(
        client_name="Acme Kft.", period_start="2026-09-21", period_end="2026-09-27", hours_logged=8.0
    )
    prompt = _build_prompt(req)
    assert "8.0" in prompt


def test_build_prompt_omits_hours_logged_when_zero():
    req = ClientStatusEmailRequest(client_name="Acme Kft.", period_start="2026-09-21", period_end="2026-09-27")
    prompt = _build_prompt(req)
    assert "Naplózott óra" not in prompt


def test_build_prompt_includes_invoice_activity_when_present():
    req = ClientStatusEmailRequest(
        client_name="Acme Kft.",
        period_start="2026-09-21",
        period_end="2026-09-27",
        invoices_created=2,
        invoices_created_total=100000.0,
        invoices_paid=1,
    )
    prompt = _build_prompt(req)
    assert "Kiállított számlák" in prompt
    assert "kiegyenlített számlák" in prompt


def test_build_prompt_omits_invoice_activity_when_zero():
    req = ClientStatusEmailRequest(client_name="Acme Kft.", period_start="2026-09-21", period_end="2026-09-27")
    prompt = _build_prompt(req)
    assert "Kiállított számlák" not in prompt
    assert "kiegyenlített számlák" not in prompt
