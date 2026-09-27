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


def test_draft_reply_works_without_email_content_for_new_message():
    # Composing a brand-new (non-reply) email has no incoming email to
    # attach - email_content is optional for exactly this case.
    with draft_reply_agent.override(model=TestModel(custom_output_args={"draft": "Új üzenet szövege."})):
        response = client.post(
            "/draft-reply",
            json={"instruction": "Írj egy rövid ajánlatkérést a beszállítónak."},
        )
    assert response.status_code == 200
    assert response.json()["draft"] == "Új üzenet szövege."


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


def test_build_prompt_includes_instruction_when_present():
    req = DraftReplyRequest(
        email_content="Szia, mikor kezdünk?",
        instruction="Mondd meg neki, hogy jövő héten kezdünk, de kérek egy hét csúszást.",
    )
    prompt = _build_prompt(req)
    assert "jövő héten kezdünk, de kérek egy hét csúszást" in prompt


def test_build_prompt_omits_instruction_section_when_empty():
    req = DraftReplyRequest(email_content="Csak egy teszt üzenet.")
    prompt = _build_prompt(req)
    assert "A felhasználó a következőt szeretné közölni" not in prompt


def test_build_prompt_omits_incoming_email_section_when_content_empty():
    req = DraftReplyRequest(instruction="Írj egy rövid ajánlatkérést.")
    prompt = _build_prompt(req)
    assert "Beérkező email:" not in prompt
    assert "Írj egy rövid ajánlatkérést." in prompt


def test_build_prompt_includes_incoming_email_section_when_content_present():
    req = DraftReplyRequest(email_content="Szia, mikor kezdünk?")
    prompt = _build_prompt(req)
    assert "Beérkező email:\nSzia, mikor kezdünk?" in prompt


def test_build_prompt_instruction_wording_differs_for_new_message_vs_reply():
    reply_req = DraftReplyRequest(email_content="Szia, mikor kezdünk?", instruction="Jövő héten.")
    new_message_req = DraftReplyRequest(instruction="Jövő héten.")
    reply_prompt = _build_prompt(reply_req)
    new_message_prompt = _build_prompt(new_message_req)
    assert "a válaszban" in reply_prompt
    assert "önálló, új e-mail" in new_message_prompt


def test_build_prompt_includes_edit_examples_section_when_present():
    req = DraftReplyRequest(
        email_content="Szia, mikor kezdünk?",
        edit_examples=[{"ai_draft": "Tisztelt Cím! Ezúton értesítem.", "sent": "Szia! Jövő héten kezdünk."}],
    )
    prompt = _build_prompt(req)
    assert "szoktad átírni" in prompt
    assert "Tisztelt Cím! Ezúton értesítem." in prompt
    assert "Szia! Jövő héten kezdünk." in prompt


def test_build_prompt_omits_edit_examples_section_when_empty():
    req = DraftReplyRequest(email_content="Csak egy teszt üzenet.")
    prompt = _build_prompt(req)
    assert "szoktad átírni" not in prompt


def test_draft_reply_accepts_explicit_null_similar_replies():
    with draft_reply_agent.override(model=TestModel(custom_output_args={"draft": "d"})):
        response = client.post(
            "/draft-reply",
            json={"email_content": "Csak egy teszt üzenet.", "similar_replies": None},
        )
    assert response.status_code == 200
