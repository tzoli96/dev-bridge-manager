import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Each agent module reads its own GEMINI_MODEL rather than importing it from
# categorize_email.py, to avoid a circular import - same reasoning as the
# comment in categorize_email.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class TaskBreakdownRequest(BaseModel):
    email_content: str
    subject: str = ""


class TaskItem(BaseModel):
    title: str
    description: str


class TaskGroup(BaseModel):
    title: str
    description: str
    subtasks: list[TaskItem] = []


class TaskBreakdownResult(BaseModel):
    groups: list[TaskGroup]


task_breakdown_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=TaskBreakdownResult,
    defer_model_check=True,
    instructions=(
        "Read a business email and identify the concrete, actionable tasks "
        "the recipient needs to do in response. Write in Hungarian unless "
        "the email is in another language. Each task needs a short, clear, "
        "actionable title (imperative mood) and a one-to-three sentence "
        "description grounded in what the email actually says - never "
        "invent tasks or details not supported by the email content.\n\n"
        "Group tasks by relationship: if several action items are really "
        "steps of the same effort (e.g. 'reply to the client' and 'attach "
        "the contract' for the same request), put one as the group's "
        "primary task and the rest as its subtasks. If action items are "
        "genuinely independent of each other, give each its own group with "
        "an empty subtasks list - do not force unrelated items together.\n\n"
        "If the email only calls for one clear action, return exactly one "
        "group with an empty subtasks list. If the email calls for no "
        "action at all, still return exactly one group summarizing it as a "
        "'review/file' style task, since every incoming email needs at "
        "least a place to track it."
    ),
)


def _build_prompt(req: TaskBreakdownRequest) -> str:
    parts = []
    if req.subject.strip():
        parts.append(f"Tárgy: {req.subject.strip()}")
    parts.append(f"Email tartalma:\n{req.email_content}")
    return "\n\n".join(parts)


async def task_breakdown(req: TaskBreakdownRequest) -> list[TaskGroup]:
    result = await task_breakdown_agent.run(_build_prompt(req))
    return result.output.groups
