import logging
import os
import time

from fastapi import FastAPI, Request

import logfire

from app.categorize_email import CategorizeEmailRequest, CategorizeEmailResult, categorize_email

logger = logging.getLogger("devbridge_ai")
# No observability existed before this: not one call logged its latency, and
# a plain getLogger() without basicConfig defaults to WARNING, so info-level
# calls below would silently never show up.
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")

app = FastAPI(title="Dev Bridge Manager AI service")

# Telemetry. `if-token-present` is what keeps this honest locally: with no
# LOGFIRE_TOKEN set - development, CI, the test suite - configure() produces a
# no-op, makes no network call, and the service behaves exactly as it did
# before. The token only exists on the box, so only the box reports.
logfire.configure(service_name="devbridge-ai", send_to_logfire="if-token-present")
logfire.instrument_fastapi(app)
logging.getLogger().addHandler(logfire.LogfireLoggingHandler())


@app.middleware("http")
async def log_request_timing(request: Request, call_next):
    started = time.monotonic()
    try:
        response = await call_next(request)
    except Exception:
        duration_ms = round((time.monotonic() - started) * 1000)
        logger.info("ai_call path=%s ok=False durationMs=%d", request.url.path, duration_ms)
        raise
    duration_ms = round((time.monotonic() - started) * 1000)
    logger.info(
        "ai_call path=%s ok=%s status=%d durationMs=%d",
        request.url.path,
        response.status_code < 400,
        response.status_code,
        duration_ms,
    )
    return response


@app.get("/health")
def health():
    # Liveness only - no live Gemini call on every poll (the docker-compose
    # healthcheck hits this every 5s). But without GEMINI_API_KEY/
    # GOOGLE_API_KEY set, every real request fails anyway; surfacing that
    # here means a misconfigured deploy shows up immediately instead of only
    # once a feature tries to use an AI-backed endpoint.
    gemini_configured = bool(os.environ.get("GEMINI_API_KEY") or os.environ.get("GOOGLE_API_KEY"))
    return {"status": "ok", "service": "ai", "geminiConfigured": gemini_configured}


@app.post("/categorize-email", response_model=CategorizeEmailResult)
async def categorize_email_endpoint(payload: CategorizeEmailRequest) -> CategorizeEmailResult:
    category = await categorize_email(payload)
    return CategorizeEmailResult(category=category)
