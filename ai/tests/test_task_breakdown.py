from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.main import app
from app.task_breakdown import TaskBreakdownRequest, _build_prompt, task_breakdown_agent

client = TestClient(app)


def test_task_breakdown_returns_groups():
    override_output = {
        "groups": [
            {
                "title": "Válaszolj az árajánlatra",
                "description": "A kliens árajánlatot kér a projektre.",
                "subtasks": [
                    {"title": "Csatold a szerződést", "description": "Küldd el a szerződéstervezetet."}
                ],
            },
            {
                "title": "Frissítsd a határidőt a naptárban",
                "description": "A kliens új határidőt javasolt.",
                "subtasks": [],
            },
        ]
    }
    with task_breakdown_agent.override(model=TestModel(custom_output_args=override_output)):
        response = client.post(
            "/task-breakdown",
            json={"email_content": "Szia, kérnék egy árajánlatot és egy új határidőt.", "subject": "Árajánlat kérés"},
        )
    assert response.status_code == 200
    body = response.json()
    assert len(body["groups"]) == 2
    assert body["groups"][0]["subtasks"][0]["title"] == "Csatold a szerződést"
    assert body["groups"][1]["subtasks"] == []


def test_task_breakdown_requires_email_content():
    response = client.post("/task-breakdown", json={})
    assert response.status_code == 422


def test_build_prompt_includes_subject_when_present():
    req = TaskBreakdownRequest(email_content="Csak egy teszt üzenet.", subject="Fontos ügy")
    prompt = _build_prompt(req)
    assert "Fontos ügy" in prompt
    assert "Csak egy teszt üzenet." in prompt


def test_build_prompt_omits_subject_section_when_empty():
    req = TaskBreakdownRequest(email_content="Csak egy teszt üzenet.")
    prompt = _build_prompt(req)
    assert "Tárgy:" not in prompt
