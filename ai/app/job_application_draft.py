import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Read GEMINI_MODEL directly in this module rather than importing it from
# another agent module, to avoid a circular import - same reasoning as
# categorize_email.py/draft_reply.py/task_breakdown.py/job_match.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class JobApplicationDraftRequest(BaseModel):
    cv_text: str
    skills: str
    job_title: str
    company: str
    job_description: str
    instruction: str = ""


class JobApplicationDraftResult(BaseModel):
    draft: str


job_application_draft_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=JobApplicationDraftResult,
    defer_model_check=True,
    instructions=(
        "Írj egy udvarias, magyar nyelvű, kész (nem placeholder) motivációs "
        "levelet egy állásjelentkezéshez, a megadott CV, készségek és "
        "álláshirdetés alapján. Emeld ki a hirdetés szempontjából releváns "
        "tapasztalatot. Ha kifejezett kiemelendő szempont van megadva, azt "
        "kezeld explicit hangsúlyként a levélben."
    ),
)


def _build_prompt(req: JobApplicationDraftRequest) -> str:
    instruction_section = f"\nKiemelendő szempont: {req.instruction}" if req.instruction.strip() else ""
    return (
        f"CV: {req.cv_text}\n"
        f"Készségek: {req.skills}"
        f"{instruction_section}\n\n"
        f"Állás: {req.job_title} - {req.company}\n"
        f"Leírás: {req.job_description}"
    )


async def job_application_draft(req: JobApplicationDraftRequest) -> str:
    result = await job_application_draft_agent.run(_build_prompt(req))
    return result.output.draft
