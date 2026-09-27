import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Each agent module reads its own GEMINI_MODEL rather than importing it from
# categorize_email.py, to avoid a circular import - same reasoning as the
# comment in categorize_email.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class DraftEditExample(BaseModel):
    ai_draft: str
    sent: str


class DraftReplyRequest(BaseModel):
    # Empty when there is no incoming email to reply to - i.e. drafting a
    # brand-new outgoing message rather than a reply.
    email_content: str = ""
    profile_context: str = ""
    similar_replies: list[str] | None = None
    edit_examples: list[DraftEditExample] | None = None
    instruction: str = ""


class DraftReplyResult(BaseModel):
    draft: str


draft_reply_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=DraftReplyResult,
    defer_model_check=True,
    instructions=(
        "Draft a business email on behalf of the user. Write in Hungarian "
        "unless the original email is in another language. This is a draft "
        "suggestion only - a human will review and edit it before sending, "
        "so prefer a complete, ready-to-edit draft over placeholders. If an "
        "incoming email is included in the prompt, this is a reply - keep "
        "it focused and only as long as the incoming email warrants. If no "
        "incoming email is included, this is a brand-new outgoing message, "
        "not a reply - do not invent an addressee or reference prior "
        "correspondence that isn't there. If the prompt includes the "
        "user's own instruction about what to say, treat that as the "
        "actual content of the message and rephrase/expand it into a "
        "polished message - do not invent content beyond it or ignore it "
        "in favor of guessing from the incoming email. If no such "
        "instruction is given, infer an appropriate reply from the "
        "incoming email as usual."
    ),
)


def _build_prompt(req: DraftReplyRequest) -> str:
    has_incoming_email = bool(req.email_content.strip())
    prompt_parts = []
    if req.profile_context.strip():
        prompt_parts.append(
            "Az alábbi profil alapján fogalmazz e-mailt a felhasználó "
            "nevében. Vedd figyelembe a hátterét, szakterületét és "
            "kommunikációs stílusát, az írásmintákat pedig "
            "hangnem-referenciaként használd.\n\n" + req.profile_context
        )
    if req.instruction.strip():
        if has_incoming_email:
            prompt_parts.append(
                "A felhasználó a következőt szeretné közölni a válaszban - "
                "ezt fogalmazd át kész, udvarias, a fenti stílushoz illő "
                "válasszá. Ne találj ki ezen felül más tartalmat, csak amit "
                "a beérkező email kontextusa (pl. megszólítás, hivatkozás a "
                "kérdésére) indokol:\n\n" + req.instruction.strip()
            )
        else:
            prompt_parts.append(
                "A felhasználó a következőt szeretné közölni egy új, nem "
                "válasz e-mailben - ezt fogalmazd át kész, udvarias "
                "üzenetté. Ne adj hozzá megszólítást vagy hivatkozást "
                "korábbi levelezésre, mivel ez egy önálló, új e-mail:"
                "\n\n" + req.instruction.strip()
            )
    if req.similar_replies:
        examples = "\n---\n".join(req.similar_replies)
        prompt_parts.append(
            "Az alábbi, korábban általad írt, hasonló témájú/címzettnek "
            "szóló válaszok stílusát is vedd figyelembe:\n\n---\n" + examples
        )
    if req.edit_examples:
        edit_pairs = "\n---\n".join(
            f"[AI piszkozat]: {e.ai_draft}\n[Elküldött verzió]: {e.sent}"
            for e in req.edit_examples
        )
        prompt_parts.append(
            "Az alábbi példák azt mutatják, hogyan szoktad átírni a korábbi "
            "AI-piszkozatokat, mielőtt elküldted őket - vedd figyelembe ezt "
            "a szerkesztési stílust:\n\n---\n" + edit_pairs
        )
    if has_incoming_email:
        prompt_parts.append(f"Beérkező email:\n{req.email_content}")
    return "\n\n".join(prompt_parts)


async def draft_reply(req: DraftReplyRequest) -> str:
    result = await draft_reply_agent.run(_build_prompt(req))
    return result.output.draft
