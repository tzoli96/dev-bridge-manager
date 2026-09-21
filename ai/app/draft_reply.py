import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Each agent module reads its own GEMINI_MODEL rather than importing it from
# categorize_email.py, to avoid a circular import - same reasoning as the
# comment in categorize_email.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class DraftReplyRequest(BaseModel):
    email_content: str
    profile_context: str = ""


class DraftReplyResult(BaseModel):
    draft: str


draft_reply_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=DraftReplyResult,
    defer_model_check=True,
    instructions=(
        "Draft a reply to a business email on behalf of the user. Write in "
        "Hungarian unless the original email is in another language. Keep "
        "the reply focused and only as long as the original email warrants. "
        "This is a draft suggestion only - a human will review and edit it "
        "before sending, so prefer a complete, ready-to-edit draft over "
        "placeholders."
    ),
)


async def draft_reply(req: DraftReplyRequest) -> str:
    prompt_parts = []
    if req.profile_context.strip():
        prompt_parts.append(
            "Az alábbi profil alapján fogalmazz választ a felhasználó nevében. "
            "Vedd figyelembe a hátterét, szakterületét és kommunikációs "
            "stílusát, az írásmintákat pedig hangnem-referenciaként "
            "használd.\n\n" + req.profile_context
        )
    prompt_parts.append(f"Beérkező email:\n{req.email_content}")

    result = await draft_reply_agent.run("\n\n".join(prompt_parts))
    return result.output.draft
