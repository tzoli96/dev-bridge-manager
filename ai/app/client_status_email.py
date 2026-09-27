import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Read GEMINI_MODEL directly in this module rather than importing it from
# another agent module, to avoid a circular import - same reasoning as
# categorize_email.py/draft_reply.py/task_breakdown.py/job_match.py/
# job_application_draft.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class ClientStatusEmailRequest(BaseModel):
    client_name: str
    period_start: str
    period_end: str
    completed_tasks: list[str] = []
    hours_logged: float = 0.0
    invoices_created: int = 0
    invoices_created_total: float = 0.0
    invoices_paid: int = 0


class ClientStatusEmailResult(BaseModel):
    subject: str
    body: str


client_status_email_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=ClientStatusEmailResult,
    defer_model_check=True,
    instructions=(
        "Írj egy rövid, barátságos, magyar nyelvű heti státusz-emailt egy "
        "ügyfélnek, a megadott tények alapján. Ez egy kész piszkozat, amit "
        "egy munkatárs átnéz és szükség esetén szerkeszt küldés előtt, ne "
        "használj helykitöltőket. Csak a megadott tényekre hivatkozz, ne "
        "találj ki plusz részletet. Ha egy adat nulla vagy hiányzik (pl. "
        "nincs naplózott óra), egyszerűen ne térj ki rá."
    ),
)


def _build_prompt(req: ClientStatusEmailRequest) -> str:
    lines = [
        f"Ügyfél: {req.client_name}",
        f"Időszak: {req.period_start} - {req.period_end}",
    ]
    if req.completed_tasks:
        lines.append("Ezen a héten elkészült feladatok:")
        lines.extend(f"- {task}" for task in req.completed_tasks)
    if req.hours_logged > 0:
        lines.append(f"Naplózott óra ezen a héten: {req.hours_logged}")
    if req.invoices_created > 0:
        lines.append(
            f"Kiállított számlák ezen a héten: {req.invoices_created} db, összesen {req.invoices_created_total} Ft"
        )
    if req.invoices_paid > 0:
        lines.append(f"Ezen a héten kiegyenlített számlák: {req.invoices_paid} db")
    return "\n".join(lines)


async def client_status_email(req: ClientStatusEmailRequest) -> ClientStatusEmailResult:
    result = await client_status_email_agent.run(_build_prompt(req))
    return result.output
