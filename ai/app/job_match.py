import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Read GEMINI_MODEL directly in this module rather than importing it from
# another agent module, to avoid a circular import - same reasoning as
# categorize_email.py/draft_reply.py/task_breakdown.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class JobMatchRequest(BaseModel):
    cv_text: str
    skills: str
    preferences: str = ""
    job_title: str
    company: str
    location: str
    job_description: str


class JobMatchResult(BaseModel):
    score: int
    reasoning: str


job_match_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=JobMatchResult,
    defer_model_check=True,
    instructions=(
        "Egy álláskereső CV-je, készségei és (opcionálisan) preferenciái "
        "alapján pontozd 0-100 skálán, mennyire illik rá az adott állás. "
        "0-20: nem releváns. 40-60: részleges egyezés. 80-100: erős egyezés. "
        "Az indoklás legyen 1-2 mondat, magyar nyelven."
    ),
)


def _build_prompt(req: JobMatchRequest) -> str:
    preferences_section = f"\nPreferenciák: {req.preferences}" if req.preferences.strip() else ""
    return (
        f"CV: {req.cv_text}\n"
        f"Készségek: {req.skills}"
        f"{preferences_section}\n\n"
        f"Állás: {req.job_title} - {req.company} ({req.location})\n"
        f"Leírás: {req.job_description}"
    )


async def job_match(req: JobMatchRequest) -> JobMatchResult:
    result = await job_match_agent.run(_build_prompt(req))
    return result.output
