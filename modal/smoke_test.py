#!/usr/bin/env python3
"""Smoke test for the deployed Sela LLM server (modal/sela_llm.py). Stdlib only.

Reads from the environment (never printed):
  SELA_LLM_BASE_URL   server root, e.g. https://<workspace>-<env>--sela-llm-serve.us-east.modal.direct
  SELA_LLM_API_KEY    vLLM API key (sent as "Authorization: Bearer")
  SELA_MODAL_KEY      Modal proxy token id (sent as "Modal-Key")
  SELA_MODAL_SECRET   Modal proxy token secret (sent as "Modal-Secret")

Checks, in order:
  1. unauthenticated_rejected: GET /health without credentials gets 401/403.
     vLLM never guards /health, so only Modal's proxy can reject it; with
     proxy auth on, this never starts a GPU container.
  2. cold_start: GET /v1/models with full credentials, retrying 503/429 with
     capped exponential backoff until the budget runs out; reports seconds.
     NOTE: this request starts a paid GPU container if none is running.
  3. models: /v1/models lists the expected served name; reports whether the
     reported "root" path contains the pinned revision.
  4. vllm_key_enforced: Modal credentials plus a wrong vLLM key gets 401 from
     vLLM. Together with 2 this verifies the two auth layers the Go client
     relies on (Modal-Key/Modal-Secret plus Authorization: Bearer).
  5. chat_stream: one streamed chat completion with a small JSON schema on a
     SYNTHETIC Indonesian sentence; reports time to first byte, total latency,
     completion tokens, reasoning deltas, and schema validity of the content.

Prints one JSON report on stdout (progress lines go to stderr). Generated
text is included only with --show-output; reasoning text never is. Exit code
0 when every check passed, 1 when a check failed, 2 on configuration errors.

Redirects are never followed: any 3xx fails the check that got it, so the
credentials above can only reach SELA_LLM_BASE_URL's host (see SmokeClient).

Model facts used here (reasoning_strength, delta.reasoning, sampling
defaults) are documented with sources in modal/MODEL_NOTES.md.
"""

from __future__ import annotations

import argparse
import datetime as _dt
import ipaddress
import json
import os
import secrets as _secrets
import socket
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

TOOL = "sela-modal-smoke"
TOOL_VERSION = 1

EXPECTED_SERVED_MODEL = "sela-glimmer-cec94fc"
EXPECTED_REVISION = "cec94fcfd6512f2cfe708138c32533b346ea4f69"

ENV_BASE_URL = "SELA_LLM_BASE_URL"
ENV_API_KEY = "SELA_LLM_API_KEY"
ENV_MODAL_KEY = "SELA_MODAL_KEY"
ENV_MODAL_SECRET = "SELA_MODAL_SECRET"
REQUIRED_ENV = (ENV_BASE_URL, ENV_API_KEY, ENV_MODAL_KEY, ENV_MODAL_SECRET)

# Modal answers 503 while no container is ready (requests do not queue) and
# 429 when a container is at max_concurrency (modal.com/docs/guide/servers).
RETRYABLE_STATUSES = frozenset({429, 503})
REASONING_STRENGTHS = ("low", "medium", "high", "xhigh")

# Synthetic test input: invented for this test, describes no real person.
SYNTHETIC_TEXT = (
    "Akhir-akhir ini saya susah tidur karena tugas kuliah menumpuk, "
    "tapi saya belum tahu harus cerita ke siapa."
)
SYSTEM_PROMPT = (
    "You extract fields for an automated software test. The user text is synthetic. "
    "Reply with exactly one JSON object that matches the given JSON schema and nothing else. "
    "exact_quote must be copied verbatim from the user text. Do not give advice."
)
SMOKE_SCHEMA = {
    "type": "object",
    "properties": {
        "language": {"type": "string", "enum": ["id", "en", "mixed"]},
        "main_concern": {"type": "string", "minLength": 1, "maxLength": 80},
        "exact_quote": {"type": "string", "minLength": 1, "maxLength": 200},
        "mentions_sleep": {"type": "boolean"},
    },
    "required": ["language", "main_concern", "exact_quote", "mentions_sleep"],
    "additionalProperties": False,
}


class ConfigError(Exception):
    """Invalid or missing configuration (exit code 2). Never carries values."""


# ---------------------------------------------------------------------------
# Pure helpers (unit tested in tests/modal/test_sela_llm_config.py).
# ---------------------------------------------------------------------------


def backoff_delays(budget_s: float, initial_s: float = 2.0, factor: float = 2.0, max_delay_s: float = 30.0):
    """Capped exponential delays whose running total never exceeds budget_s.

    The last delay is shortened so the total lands exactly on the budget.
    """
    if budget_s <= 0 or initial_s <= 0:
        return
    total = 0.0
    delay = initial_s
    while total < budget_s:
        step = min(delay, max_delay_s, budget_s - total)
        if step <= 0:
            return
        total += step
        yield step
        delay = min(delay * factor, max_delay_s)


def _type_matches(value, expected: str) -> bool:
    if expected == "object":
        return isinstance(value, dict)
    if expected == "array":
        return isinstance(value, list)
    if expected == "string":
        return isinstance(value, str)
    if expected == "boolean":
        return isinstance(value, bool)
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if expected == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    if expected == "null":
        return value is None
    return False


def validate_json_schema(instance, schema: dict, path: str = "$") -> list[str]:
    """Validate a subset of JSON Schema. Errors name paths, never values.

    Supports: type (string or list), enum, const, properties, required,
    additionalProperties (bool or schema), minLength, maxLength, minimum,
    maximum, items, minItems, maxItems.
    """
    errors: list[str] = []
    expected_type = schema.get("type")
    if expected_type is not None:
        types = expected_type if isinstance(expected_type, list) else [expected_type]
        if not any(_type_matches(instance, t) for t in types):
            return [f"{path}: expected type {'/'.join(types)}"]
    if "enum" in schema and instance not in schema["enum"]:
        errors.append(f"{path}: value not in enum")
    if "const" in schema and instance != schema["const"]:
        errors.append(f"{path}: value differs from const")
    if isinstance(instance, str):
        if "minLength" in schema and len(instance) < schema["minLength"]:
            errors.append(f"{path}: shorter than minLength {schema['minLength']}")
        if "maxLength" in schema and len(instance) > schema["maxLength"]:
            errors.append(f"{path}: longer than maxLength {schema['maxLength']}")
    if isinstance(instance, (int, float)) and not isinstance(instance, bool):
        if "minimum" in schema and instance < schema["minimum"]:
            errors.append(f"{path}: below minimum")
        if "maximum" in schema and instance > schema["maximum"]:
            errors.append(f"{path}: above maximum")
    if isinstance(instance, dict):
        properties = schema.get("properties", {})
        for key in schema.get("required", []):
            if key not in instance:
                errors.append(f"{path}: missing required property {key!r}")
        additional = schema.get("additionalProperties", True)
        for key, value in instance.items():
            child = f"{path}.{key}"
            if key in properties:
                errors.extend(validate_json_schema(value, properties[key], child))
            elif additional is False:
                errors.append(f"{path}: unexpected property {key!r}")
            elif isinstance(additional, dict):
                errors.extend(validate_json_schema(value, additional, child))
    if isinstance(instance, list):
        if "minItems" in schema and len(instance) < schema["minItems"]:
            errors.append(f"{path}: fewer than minItems")
        if "maxItems" in schema and len(instance) > schema["maxItems"]:
            errors.append(f"{path}: more than maxItems")
        if isinstance(schema.get("items"), dict):
            for index, item in enumerate(instance):
                errors.extend(validate_json_schema(item, schema["items"], f"{path}[{index}]"))
    return errors


class StreamAccumulator:
    """Consumes an OpenAI-compatible SSE stream line by line.

    vLLM 0.32 puts reasoning in delta.reasoning; SGLang/llama.cpp (and older
    vLLM) use delta.reasoning_content. Both are counted and the field names
    seen are reported. Reasoning text is counted, never kept.
    """

    def __init__(self, clock=time.monotonic, started: float | None = None):
        self._clock = clock
        self.started = clock() if started is None else started
        self._data_lines: list[str] = []
        self.content_parts: list[str] = []
        self.content_deltas = 0
        self.reasoning_deltas = 0
        self.reasoning_chars = 0
        self.reasoning_fields: set[str] = set()
        self.first_reasoning_s: float | None = None
        self.first_content_s: float | None = None
        self.finish_reason: str | None = None
        self.usage: dict | None = None
        self.models: set[str] = set()
        self.errors: list[str] = []
        self.done = False
        self.malformed_events = 0
        self.events = 0

    @property
    def content(self) -> str:
        return "".join(self.content_parts)

    def feed_line(self, line: str) -> None:
        line = line.rstrip("\r\n")
        if line == "":
            self._dispatch()
            return
        if line.startswith(":"):
            return  # SSE comment (e.g. keep-alive)
        field, _, value = line.partition(":")
        if value.startswith(" "):
            value = value[1:]
        if field == "data":
            self._data_lines.append(value)

    def close(self) -> None:
        self._dispatch()

    def _dispatch(self) -> None:
        if not self._data_lines:
            return
        data = "\n".join(self._data_lines)
        self._data_lines = []
        if data.strip() == "[DONE]":
            self.done = True
            return
        try:
            chunk = json.loads(data)
        except ValueError:
            self.malformed_events += 1
            return
        if not isinstance(chunk, dict):
            self.malformed_events += 1
            return
        self.events += 1
        if "error" in chunk:
            error = chunk["error"]
            message = error.get("message") if isinstance(error, dict) else error
            self.errors.append(str(message)[:300])
        if isinstance(chunk.get("model"), str):
            self.models.add(chunk["model"])
        if isinstance(chunk.get("usage"), dict):
            self.usage = chunk["usage"]
        for choice in chunk.get("choices") or []:
            delta = choice.get("delta") or {}
            for name in ("reasoning", "reasoning_content"):
                text = delta.get(name)
                if isinstance(text, str) and text:
                    self.reasoning_deltas += 1
                    self.reasoning_chars += len(text)
                    self.reasoning_fields.add(name)
                    if self.first_reasoning_s is None:
                        self.first_reasoning_s = self._clock() - self.started
            text = delta.get("content")
            if isinstance(text, str) and text:
                self.content_deltas += 1
                self.content_parts.append(text)
                if self.first_content_s is None:
                    self.first_content_s = self._clock() - self.started
            if choice.get("finish_reason"):
                self.finish_reason = choice["finish_reason"]


def redact(value, secret_values):
    """Replace every secret occurrence in strings nested inside value."""
    secret_values = [s for s in secret_values if s]
    if isinstance(value, str):
        for secret in secret_values:
            value = value.replace(secret, "[REDACTED]")
        return value
    if isinstance(value, dict):
        return {key: redact(item, secret_values) for key, item in value.items()}
    if isinstance(value, list):
        return [redact(item, secret_values) for item in value]
    return value


def build_report(
    checks: dict,
    target: dict,
    started_at: str,
    finished_at: str,
    secret_values=(),
    content: str | None = None,
    show_output: bool = False,
) -> dict:
    """Assemble the JSON report. Generated content only when show_output."""
    report = {
        "tool": TOOL,
        "tool_version": TOOL_VERSION,
        "started_at": started_at,
        "finished_at": finished_at,
        "target": target,
        "ok": bool(checks) and all(check.get("ok") is True for check in checks.values()),
        "checks": checks,
    }
    if show_output:
        report["output"] = {"content": content}
    return redact(report, list(secret_values))


def parse_base_url(raw: str) -> str:
    """Validate the server root URL; https required except for loopback."""
    parsed = urllib.parse.urlsplit(raw.strip())
    if parsed.scheme not in ("https", "http") or not parsed.hostname:
        raise ConfigError(f"{ENV_BASE_URL} must be an absolute https URL")
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ConfigError(f"{ENV_BASE_URL} must not contain credentials, a query or a fragment")
    if parsed.scheme == "http" and not _is_loopback(parsed.hostname):
        raise ConfigError(f"{ENV_BASE_URL} must use https (http is allowed only for loopback tests)")
    return urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, parsed.path.rstrip("/"), "", ""))


def _is_loopback(host: str) -> bool:
    if host == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


# ---------------------------------------------------------------------------
# HTTP client
# ---------------------------------------------------------------------------


class HttpResult:
    def __init__(self, status: int | None, body: bytes = b"", error: str | None = None, fatal: bool = False):
        self.status = status
        self.body = body
        self.error = error
        # True for failures that waiting cannot fix (DNS, TLS verification).
        self.fatal = fatal

    def json(self):
        try:
            return json.loads(self.body.decode("utf-8"))
        except (ValueError, UnicodeDecodeError):
            return None


class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Refuses every redirect, so credentials never leave the configured host.

    urllib's stock HTTPRedirectHandler re-sends every header added with
    Request(headers=...)/add_header() to the redirect target ("headers added
    using this method are also added to redirected requests"), whatever its
    host or scheme, https->http included. Raising HTTPError is the documented
    way to say "no other handler should try to handle this URL"; the 3xx then
    reaches the caller as an HTTPError and the check fails. Source checked
    2026-10-26: https://docs.python.org/3/library/urllib.request.html
    (build_opener, HTTPRedirectHandler.redirect_request, Request.add_header).
    The Go client takes the same stance (internal/llm/provider.go httpClient:
    CheckRedirect returns http.ErrUseLastResponse).
    """

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise urllib.error.HTTPError(req.full_url, code, f"redirect not followed: {msg}", headers, fp)


class SmokeClient:
    """Minimal client. auth: "none" | "proxy_wrong_key" | "full".

    Two independent guards keep credentials on the configured host:
    1. the opener refuses redirects (NoRedirectHandler replaces urllib's stock
       handler: build_opener skips a default handler when a subclass of it is
       passed), for loopback and remote hosts alike;
    2. credentials are attached with add_unredirected_header(), which urllib
       never copies onto a redirected request, so even an opener that follows
       redirects would not forward them.
    """

    def __init__(self, base_url: str, api_key: str, modal_key: str, modal_secret: str, timeout_s: float):
        self.base_url = base_url
        self.host = urllib.parse.urlsplit(base_url).hostname or ""
        self._api_key = api_key
        self._modal_key = modal_key
        self._modal_secret = modal_secret
        self.timeout_s = timeout_s
        handlers = [NoRedirectHandler()]
        if _is_loopback(self.host):
            handlers.append(urllib.request.ProxyHandler({}))  # never proxy loopback
        self._opener = urllib.request.build_opener(*handlers)

    def _headers(self, streaming: bool) -> dict:
        return {"User-Agent": f"{TOOL}/{TOOL_VERSION}", "Accept": "text/event-stream" if streaming else "application/json"}

    def _credential_headers(self, auth: str) -> dict:
        headers = {}
        if auth in ("full", "proxy_wrong_key"):
            headers["Modal-Key"] = self._modal_key
            headers["Modal-Secret"] = self._modal_secret
        if auth == "full":
            headers["Authorization"] = f"Bearer {self._api_key}"
        elif auth == "proxy_wrong_key":
            headers["Authorization"] = f"Bearer wrong-{_secrets.token_hex(16)}"
        return headers

    def _request(self, method: str, path: str, auth: str, payload=None, streaming: bool = False):
        headers = self._headers(streaming)
        data = None
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        request = urllib.request.Request(self.base_url + path, data=data, headers=headers, method=method)
        for name, value in self._credential_headers(auth).items():
            request.add_unredirected_header(name, value)
        return request

    def get(self, path: str, auth: str) -> HttpResult:
        request = self._request("GET", path, auth)
        try:
            with self._opener.open(request, timeout=self.timeout_s) as response:
                return HttpResult(response.status, response.read(1 << 20))
        except urllib.error.HTTPError as exc:
            return _http_error_result(exc)
        except (urllib.error.URLError, OSError) as exc:
            return _network_failure(exc)

    def open_stream(self, path: str, payload: dict):
        """Returns (response, None) or (None, HttpResult) on HTTP/network error."""
        request = self._request("POST", path, "full", payload, streaming=True)
        try:
            return self._opener.open(request, timeout=self.timeout_s), None
        except urllib.error.HTTPError as exc:
            return None, _http_error_result(exc)
        except (urllib.error.URLError, OSError) as exc:
            return None, _network_failure(exc)


def _http_error_result(exc: urllib.error.HTTPError) -> HttpResult:
    """HttpResult for a non-2xx answer; a 3xx gets a note naming only the
    redirect target's scheme and host (never its path, query or userinfo)."""
    error = None
    if 300 <= exc.code < 400:
        location = exc.headers.get("Location", "") if exc.headers is not None else ""
        try:
            target = urllib.parse.urlsplit(location)
            where = f" to {target.scheme}://{target.hostname}"[:120] if target.scheme and target.hostname else ""
        except ValueError:
            where = ""
        error = f"redirect not followed (HTTP {exc.code}{where}); credentials are never sent to a redirect target"
    return HttpResult(exc.code, _read_limited(exc), error=error)


def _network_failure(exc: Exception) -> HttpResult:
    reason = getattr(exc, "reason", exc)
    fatal = isinstance(reason, (socket.gaierror, ssl.SSLError))
    return HttpResult(None, error=f"{type(reason).__name__}: {str(reason)[:200]}", fatal=fatal)


def _read_limited(exc: urllib.error.HTTPError) -> bytes:
    try:
        return exc.read(4096)
    except OSError:
        return b""
    finally:
        exc.close()


def _error_summary(result: HttpResult) -> str | None:
    if result.error:
        return result.error
    payload = result.json()
    if isinstance(payload, dict):
        error = payload.get("error", payload.get("message"))
        if isinstance(error, dict):
            error = error.get("message") or error.get("type")
        if error is not None:
            return str(error)[:300]
    text = result.body.decode("utf-8", "replace").strip()
    return text[:300] or None


# ---------------------------------------------------------------------------
# Checks
# ---------------------------------------------------------------------------


def _log(message: str) -> None:
    print(f"[smoke] {message}", file=sys.stderr, flush=True)


def check_unauthenticated(client: SmokeClient) -> dict:
    """Probe /health without credentials.

    vLLM never guards /health with its API key, so only Modal's proxy can
    reject this request: 401/403 proves proxy auth is on. (A /v1 path would
    also get 401 from vLLM itself and could hide a public endpoint.) With
    proxy auth on, Modal rejects it before routing, so no container starts.
    """
    result = client.get("/health", auth="none")
    return {"ok": result.status in (401, 403), "status": result.status, "path": "/health", "error": result.error}


def wait_for_ready(client: SmokeClient, budget_s: float, sleep=time.sleep, clock=time.monotonic) -> tuple[dict, HttpResult]:
    """Poll /v1/models (full auth) until 200, retrying 503/429 and network errors."""
    started = clock()
    status_counts: dict[str, int] = {}
    attempts = 0
    delays = backoff_delays(budget_s)
    while True:
        attempts += 1
        result = client.get("/v1/models", auth="full")
        key = str(result.status) if result.status is not None else "network_error"
        status_counts[key] = status_counts.get(key, 0) + 1
        elapsed = clock() - started
        if result.status == 200:
            return {
                "ok": True,
                "cold_start_seconds": round(elapsed, 2),
                "attempts": attempts,
                "status_counts": status_counts,
                "budget_seconds": budget_s,
            }, result
        retryable = result.status in RETRYABLE_STATUSES or (result.status is None and not result.fatal)
        delay = next(delays, None) if retryable else None
        if delay is None:
            summary = {
                "ok": False,
                "attempts": attempts,
                "status_counts": status_counts,
                "budget_seconds": budget_s,
                "waited_seconds": round(elapsed, 2),
                "last_status": result.status,
                "reason": "budget exhausted" if retryable else "non-retryable status",
                "error": _error_summary(result),
            }
            return summary, result
        _log(f"not ready (status {key}); attempt {attempts}, retrying in {delay:.0f}s")
        sleep(delay)


def check_models(result: HttpResult, expected_model: str, expected_revision: str) -> dict:
    payload = result.json()
    entries = payload.get("data") if isinstance(payload, dict) else None
    if not isinstance(entries, list):
        return {"ok": False, "error": "unexpected /v1/models payload"}
    served = [entry.get("id") for entry in entries if isinstance(entry, dict)]
    match = next((entry for entry in entries if isinstance(entry, dict) and entry.get("id") == expected_model), None)
    root = match.get("root") if match else None
    return {
        "ok": match is not None,
        "served_models": served,
        "expected_model": expected_model,
        "root_contains_revision": bool(isinstance(root, str) and expected_revision in root),
        "max_model_len": match.get("max_model_len") if match else None,
    }


def check_vllm_key(client: SmokeClient) -> dict:
    result = client.get("/v1/models", auth="proxy_wrong_key")
    return {"ok": result.status == 401, "status": result.status, "error": result.error}


def build_chat_payload(expected_model: str, max_tokens: int, reasoning_strength: str, extra_body: dict | None) -> dict:
    payload = {
        "model": expected_model,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": SYNTHETIC_TEXT},
        ],
        "stream": True,
        "stream_options": {"include_usage": True},
        "max_tokens": max_tokens,
        "seed": 7,
        # No temperature/top_p/top_k: the server applies the checkpoint's
        # generation_config (1.0 / 0.95 / 64). Meta advises against greedy.
        "response_format": {
            "type": "json_schema",
            "json_schema": {"name": "sela_smoke", "schema": SMOKE_SCHEMA, "strict": True},
        },
        # Muse Glimmer's chat template reads reasoning_strength (not
        # reasoning_effort, which this template ignores).
        "chat_template_kwargs": {"reasoning_strength": reasoning_strength},
    }
    if extra_body:
        payload.update(extra_body)
    return payload


def check_chat_stream(
    client: SmokeClient,
    payload: dict,
    expected_model: str,
    deadline_s: float,
    retry_budget_s: float,
    sleep=time.sleep,
    clock=time.monotonic,
) -> tuple[dict, str]:
    """Run one streamed completion. Returns (check, content)."""
    delays = backoff_delays(retry_budget_s)
    attempts = 0
    while True:
        attempts += 1
        started = clock()
        response, failure = client.open_stream("/v1/chat/completions", payload)
        if response is not None:
            break
        retryable = failure.status in RETRYABLE_STATUSES
        delay = next(delays, None) if retryable else None
        if delay is None:
            return {
                "ok": False,
                "status": failure.status,
                "attempts": attempts,
                "error": _error_summary(failure),
            }, ""
        _log(f"chat request got {failure.status}; retrying in {delay:.0f}s")
        sleep(delay)

    accumulator = StreamAccumulator(clock=clock, started=started)
    ttfb_s = None
    timed_out = False
    status = response.status
    try:
        with response:
            for raw in response:
                if ttfb_s is None:
                    ttfb_s = clock() - started
                accumulator.feed_line(raw.decode("utf-8", "replace"))
                if accumulator.done:
                    break
                if clock() - started > deadline_s:
                    timed_out = True
                    break
    except (OSError, ValueError) as exc:
        accumulator.errors.append(f"stream read failed: {type(exc).__name__}")
    accumulator.close()
    total_s = clock() - started

    content = accumulator.content
    parsed = None
    json_ok = False
    try:
        parsed = json.loads(content)
        json_ok = True
    except ValueError:
        pass
    schema_errors = validate_json_schema(parsed, SMOKE_SCHEMA) if json_ok else ["content is not valid JSON"]
    usage = accumulator.usage or {}
    model_matches = accumulator.models == {expected_model}
    quality = None
    if json_ok and isinstance(parsed, dict):
        quote = parsed.get("exact_quote")
        quality = {
            "language_is_id": parsed.get("language") == "id",
            "mentions_sleep": parsed.get("mentions_sleep") is True,
            "quote_is_verbatim": isinstance(quote, str) and bool(quote) and quote in SYNTHETIC_TEXT,
        }
    check = {
        "ok": (
            status == 200
            and not timed_out
            and accumulator.done
            and not accumulator.errors
            and json_ok
            and not schema_errors
            and model_matches
        ),
        "status": status,
        "attempts": attempts,
        "ttfb_seconds": _round(ttfb_s),
        "first_reasoning_seconds": _round(accumulator.first_reasoning_s),
        "first_content_seconds": _round(accumulator.first_content_s),
        "total_seconds": _round(total_s),
        "timed_out": timed_out,
        "stream_done": accumulator.done,
        "finish_reason": accumulator.finish_reason,
        "prompt_tokens": usage.get("prompt_tokens"),
        "completion_tokens": usage.get("completion_tokens"),
        "reasoning_present": accumulator.reasoning_deltas > 0,
        "reasoning_deltas": accumulator.reasoning_deltas,
        "reasoning_chars": accumulator.reasoning_chars,
        "reasoning_fields": sorted(accumulator.reasoning_fields),
        "content_deltas": accumulator.content_deltas,
        "content_chars": len(content),
        "json_parse_ok": json_ok,
        "schema_valid": json_ok and not schema_errors,
        "schema_errors": schema_errors[:10],
        "models_reported": sorted(accumulator.models),
        "model_matches": model_matches,
        "stream_errors": accumulator.errors[:5],
        "malformed_events": accumulator.malformed_events,
        "quality_signals": quality,
        "reasoning_strength": (payload.get("chat_template_kwargs") or {}).get("reasoning_strength"),
    }
    return check, content


def _round(value):
    return None if value is None else round(value, 3)


def _now() -> str:
    return _dt.datetime.now(_dt.timezone.utc).isoformat(timespec="seconds")


# ---------------------------------------------------------------------------
# Orchestration
# ---------------------------------------------------------------------------


def run_smoke(config: dict, sleep=time.sleep, clock=time.monotonic) -> tuple[dict, int]:
    """Run all checks. config keys: base_url, api_key, modal_key, modal_secret,
    expected_model, expected_revision, cold_start_budget_s, request_timeout_s,
    chat_deadline_s, max_tokens, reasoning_strength, extra_body, show_output,
    skip_chat. Returns (report, exit_code)."""
    started_at = _now()
    secret_values = [config["api_key"], config["modal_key"], config["modal_secret"]]
    client = SmokeClient(
        config["base_url"], config["api_key"], config["modal_key"], config["modal_secret"], config["request_timeout_s"]
    )
    target = {
        "host": client.host,
        "expected_model": config["expected_model"],
        "expected_revision": config["expected_revision"],
    }
    checks: dict = {}
    content = None

    _log("checking that unauthenticated requests are rejected")
    checks["unauthenticated_rejected"] = check_unauthenticated(client)

    _log(f"waiting for the server (cold-start budget {config['cold_start_budget_s']:.0f}s; may start a GPU)")
    checks["cold_start"], ready = wait_for_ready(client, config["cold_start_budget_s"], sleep=sleep, clock=clock)
    if checks["cold_start"]["ok"]:
        checks["models"] = check_models(ready, config["expected_model"], config["expected_revision"])
        checks["vllm_key_enforced"] = check_vllm_key(client)
        if not config.get("skip_chat"):
            _log("running one streamed chat completion")
            payload = build_chat_payload(
                config["expected_model"], config["max_tokens"], config["reasoning_strength"], config.get("extra_body")
            )
            checks["chat_stream"], content = check_chat_stream(
                client,
                payload,
                config["expected_model"],
                deadline_s=config["chat_deadline_s"],
                retry_budget_s=min(120.0, config["cold_start_budget_s"]),
                sleep=sleep,
                clock=clock,
            )
    else:
        for name in ("models", "vllm_key_enforced") + (() if config.get("skip_chat") else ("chat_stream",)):
            checks[name] = {"ok": False, "skipped": "server never became ready"}

    report = build_report(
        checks,
        target,
        started_at,
        _now(),
        secret_values=secret_values,
        content=content,
        show_output=config.get("show_output", False),
    )
    return report, 0 if report["ok"] else 1


def config_from_args(argv=None, environ=None) -> dict:
    environ = os.environ if environ is None else environ
    parser = argparse.ArgumentParser(description="Smoke-test the deployed Sela LLM server.")
    parser.add_argument("--cold-start-budget", type=float, default=1200.0, help="seconds to wait for a cold start (default 1200)")
    parser.add_argument("--request-timeout", type=float, default=120.0, help="socket timeout per request in seconds")
    parser.add_argument("--chat-deadline", type=float, default=600.0, help="total seconds allowed for the streamed completion")
    parser.add_argument("--max-tokens", type=int, default=2048, help="max_tokens (reasoning counts against it)")
    parser.add_argument("--reasoning-strength", choices=REASONING_STRENGTHS, default="low")
    parser.add_argument(
        "--expected-model", default=environ.get("SELA_LLM_SERVED_MODEL") or EXPECTED_SERVED_MODEL
    )
    parser.add_argument(
        "--expected-revision", default=environ.get("SELA_LLM_MODEL_REVISION") or EXPECTED_REVISION
    )
    parser.add_argument("--extra-body", default=None, help="JSON object merged into the chat request (experiments)")
    parser.add_argument("--skip-chat", action="store_true", help="only check auth, cold start and /v1/models")
    parser.add_argument("--show-output", action="store_true", help="include the generated content in the report")
    args = parser.parse_args(argv)

    missing = [name for name in REQUIRED_ENV if not environ.get(name)]
    if missing:
        raise ConfigError("missing environment variables: " + ", ".join(missing))
    extra_body = None
    if args.extra_body:
        try:
            extra_body = json.loads(args.extra_body)
        except ValueError as exc:
            raise ConfigError("--extra-body must be a JSON object") from exc
        if not isinstance(extra_body, dict) or "stream" in extra_body:
            raise ConfigError("--extra-body must be a JSON object and must not change 'stream'")
    if args.cold_start_budget < 0 or args.max_tokens <= 0:
        raise ConfigError("--cold-start-budget must be >= 0 and --max-tokens > 0")
    return {
        "base_url": parse_base_url(environ[ENV_BASE_URL]),
        "api_key": environ[ENV_API_KEY],
        "modal_key": environ[ENV_MODAL_KEY],
        "modal_secret": environ[ENV_MODAL_SECRET],
        "expected_model": args.expected_model,
        "expected_revision": args.expected_revision,
        "cold_start_budget_s": args.cold_start_budget,
        "request_timeout_s": args.request_timeout,
        "chat_deadline_s": args.chat_deadline,
        "max_tokens": args.max_tokens,
        "reasoning_strength": args.reasoning_strength,
        "extra_body": extra_body,
        "show_output": args.show_output,
        "skip_chat": args.skip_chat,
    }


def main(argv=None, environ=None, stdout=None) -> int:
    stdout = sys.stdout if stdout is None else stdout
    try:
        config = config_from_args(argv, environ)
    except ConfigError as exc:
        print(json.dumps({"tool": TOOL, "ok": False, "config_error": str(exc)}), file=stdout)
        return 2
    report, code = run_smoke(config)
    print(json.dumps(report, indent=2, sort_keys=True), file=stdout)
    return code


if __name__ == "__main__":
    sys.exit(main())
