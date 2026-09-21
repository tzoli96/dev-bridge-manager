import os
from typing import Literal

from pydantic import BaseModel
from pydantic_ai import Agent

# Kept in sync manually with backend/internal/models/email.go's
# ValidEmailCategories - the two live in different languages, so there's
# no single source of truth to import from.
EmailCategory = Literal["ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"]

# Moved here from main.py: this Agent is the only consumer of the model
# name, and main.py importing it back would create a circular import.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class CategorizeEmailRequest(BaseModel):
    subject: str
    snippet: str
    from_address: str
    from_name: str


class CategorizeEmailResult(BaseModel):
    category: EmailCategory


categorize_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=CategorizeEmailResult,
    defer_model_check=True,
    instructions=(
        "Categorize a business email into exactly one category based on its "
        "subject, snippet, and sender.\n"
        "- 'ugyfel': a client/customer writing about project work.\n"
        "- 'szamla': invoicing, billing, or payment related.\n"
        "- 'marketing': newsletters, promotions, advertising.\n"
        "- 'rendszeruzenet': an automated notification from a tool or "
        "service, not a person.\n"
        "- 'egyeb': anything that doesn't clearly fit the above."
    ),
)


async def categorize_email(req: CategorizeEmailRequest) -> EmailCategory:
    prompt = (
        f"From: {req.from_name} <{req.from_address}>\n"
        f"Subject: {req.subject}\n"
        f"Snippet: {req.snippet}"
    )
    result = await categorize_agent.run(prompt)
    return result.output.category
