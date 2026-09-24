from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.main import app
from app.job_application_draft import JobApplicationDraftRequest, _build_prompt, job_application_draft_agent

client = TestClient(app)


def test_job_application_draft_returns_generated_draft():
    with job_application_draft_agent.override(model=TestModel(custom_output_args={"draft": "Tisztelt Cím! ..."})):
        response = client.post(
            "/job-application-draft",
            json={
                "cv_text": "5 év Go fejlesztői tapasztalat.",
                "skills": "Go, PostgreSQL, Docker",
                "job_title": "Senior Backend Engineer",
                "company": "Acme Corp",
                "job_description": "Go szolgáltatások fejlesztése és karbantartása.",
            },
        )
    assert response.status_code == 200
    assert response.json()["draft"] == "Tisztelt Cím! ..."


def test_job_application_draft_requires_job_fields():
    response = client.post("/job-application-draft", json={"cv_text": "x", "skills": "y"})
    assert response.status_code == 422


def test_build_prompt_includes_instruction_when_present():
    req = JobApplicationDraftRequest(
        cv_text="CV szöveg",
        skills="Go",
        job_title="Fejlesztő",
        company="Acme",
        job_description="Leírás",
        instruction="Emeld ki a Go tapasztalatot",
    )
    prompt = _build_prompt(req)
    assert "Emeld ki a Go tapasztalatot" in prompt


def test_build_prompt_omits_instruction_section_when_empty():
    req = JobApplicationDraftRequest(
        cv_text="CV szöveg",
        skills="Go",
        job_title="Fejlesztő",
        company="Acme",
        job_description="Leírás",
    )
    prompt = _build_prompt(req)
    assert "Kiemelendő szempont" not in prompt
