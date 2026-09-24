from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.main import app
from app.job_match import JobMatchRequest, _build_prompt, job_match_agent

client = TestClient(app)


def test_job_match_returns_score_and_reasoning():
    with job_match_agent.override(model=TestModel(custom_output_args={"score": 85, "reasoning": "Erős egyezés a Go tapasztalat miatt."})):
        response = client.post(
            "/job-match",
            json={
                "cv_text": "5 év Go fejlesztői tapasztalat.",
                "skills": "Go, PostgreSQL, Docker",
                "job_title": "Senior Backend Engineer",
                "company": "Acme Corp",
                "location": "Budapest",
                "job_description": "Go szolgáltatások fejlesztése és karbantartása.",
            },
        )
    assert response.status_code == 200
    body = response.json()
    assert body["score"] == 85
    assert body["reasoning"] == "Erős egyezés a Go tapasztalat miatt."


def test_job_match_requires_job_fields():
    response = client.post("/job-match", json={"cv_text": "x", "skills": "y"})
    assert response.status_code == 422


def test_build_prompt_includes_preferences_when_present():
    req = JobMatchRequest(
        cv_text="CV szöveg",
        skills="Go",
        preferences="Csak távmunka",
        job_title="Fejlesztő",
        company="Acme",
        location="Budapest",
        job_description="Leírás",
    )
    prompt = _build_prompt(req)
    assert "Csak távmunka" in prompt


def test_build_prompt_omits_preferences_section_when_empty():
    req = JobMatchRequest(
        cv_text="CV szöveg",
        skills="Go",
        job_title="Fejlesztő",
        company="Acme",
        location="Budapest",
        job_description="Leírás",
    )
    prompt = _build_prompt(req)
    assert "Preferenciák:" not in prompt
