import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Same model choice as categorize_email.py - this is a similarly cheap
# classification call, not a generation task.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class ClientCandidate(BaseModel):
    id: int
    name: str
    email: str


class MatchEmailClientRequest(BaseModel):
    subject: str
    snippet: str
    from_address: str
    from_name: str
    clients: list[ClientCandidate]


class MatchEmailClientResult(BaseModel):
    client_id: int | None


match_email_client_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=MatchEmailClientResult,
    defer_model_check=True,
    instructions=(
        "You are given an email already known to be from a client, plus a "
        "list of existing clients (id, name, email). Decide which client "
        "this email is from, if any.\n"
        "The sender's address does not have to exactly match a client's "
        "email - a client may write from a colleague's address or from a "
        "different address on the same company domain, and the name may be "
        "a person's name at the client company rather than the company "
        "name itself.\n"
        "Return that client's id in client_id if you are confident, or "
        "null if no client is a clear match."
    ),
)


async def match_email_client(req: MatchEmailClientRequest) -> int | None:
    if not req.clients:
        return None

    candidate_lines = "\n".join(f"- id={c.id}, name={c.name}, email={c.email}" for c in req.clients)
    prompt = (
        f"From: {req.from_name} <{req.from_address}>\n"
        f"Subject: {req.subject}\n"
        f"Snippet: {req.snippet}\n\n"
        f"Existing clients:\n{candidate_lines}"
    )
    result = await match_email_client_agent.run(prompt)
    client_id = result.output.client_id
    valid_ids = {c.id for c in req.clients}
    if client_id not in valid_ids:
        return None
    return client_id
