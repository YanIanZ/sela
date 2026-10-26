"""Offline tests for the Modal serving app and its smoke test.

modal/sela_llm.py is checked by parsing it with ast; the `modal` package is
never imported (CI runs these tests without it). Pure helpers from that file
(command builder, snapshot verification, health wait) are executed from the
AST after dropping every statement that touches `modal` or `app`.

modal/smoke_test.py is stdlib-only and is exercised against a local
http.server fake that imitates Modal's proxy (401 without proxy credentials,
503 while cold) and vLLM (401 on a wrong API key, SSE streaming with
reasoning deltas). SmokeRedirectTest adds a second local server on another
host:port and checks that redirects (301/302/303/307/308) are never
followed and that credentials never reach the redirect target.
"""

import ast
import contextlib
import hashlib
import io
import json
import os
import re
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MODAL_DIR = ROOT / "modal"
APP_PATH = MODAL_DIR / "sela_llm.py"
SMOKE_PATH = MODAL_DIR / "smoke_test.py"
README_PATH = MODAL_DIR / "README.md"
NOTES_PATH = MODAL_DIR / "MODEL_NOTES.md"
REQUIREMENTS_PATH = MODAL_DIR / "requirements.txt"
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "modal-deploy.yml"
QUALITY_WORKFLOW_PATH = ROOT / ".github" / "workflows" / "quality.yml"

sys.path.insert(0, str(MODAL_DIR))
import smoke_test  # noqa: E402  (stdlib only, asserted below)

PINNED_REVISION = "cec94fcfd6512f2cfe708138c32533b346ea4f69"
PINNED_SERVED_NAME = "sela-glimmer-cec94fc"

# Credentials used only against the local fake server. Deliberately not
# shaped like real Modal/Hugging Face tokens.
FAKE_API_KEY = "test-vllm-key-" + "0123456789abcdef" * 2
FAKE_MODAL_KEY = "test-proxy-id-" + "a" * 16
FAKE_MODAL_SECRET = "test-proxy-secret-" + "b" * 16

TOKEN_PATTERNS = [
    re.compile(r"\b(?:wk|ws|ak|as)-[A-Za-z0-9]{8,}"),  # Modal proxy / API tokens
    re.compile(r"\bhf_[A-Za-z0-9]{20,}"),  # Hugging Face tokens
    re.compile(r"\bsk-[A-Za-z0-9]{20,}"),
]
EMAIL_PATTERN = re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}")


# ---------------------------------------------------------------------------
# AST helpers for modal/sela_llm.py
# ---------------------------------------------------------------------------


def _names_in(node):
    return {sub.id for sub in ast.walk(node) if isinstance(sub, ast.Name)}


def _load_pure_namespace(tree):
    """Execute every top-level statement that does not reference modal/app."""
    body = []
    for node in tree.body:
        if isinstance(node, ast.Import):
            if any(alias.name.split(".")[0] == "modal" for alias in node.names):
                continue
        elif isinstance(node, ast.ImportFrom):
            if (node.module or "").split(".")[0] == "modal":
                continue
        elif _names_in(node) & {"modal", "app"}:
            continue
        body.append(node)
    namespace = {"__name__": "sela_llm_pure"}
    exec(compile(ast.Module(body=body, type_ignores=[]), str(APP_PATH), "exec"), namespace)
    return namespace


def _decorator_call(tree, attr):
    """The `app.<attr>(...)` decorator Call nodes in the module."""
    calls = []
    for node in tree.body:
        for decorator in getattr(node, "decorator_list", []):
            if (
                isinstance(decorator, ast.Call)
                and isinstance(decorator.func, ast.Attribute)
                and decorator.func.attr == attr
                and isinstance(decorator.func.value, ast.Name)
                and decorator.func.value.id == "app"
            ):
                calls.append((node, decorator))
    return calls


def _kwargs(call):
    return {keyword.arg: keyword.value for keyword in call.keywords}


def _modal_secret_calls(tree):
    """(method, Call) for every modal.Secret.<method>(...) call."""
    found = []
    for node in ast.walk(tree):
        if (
            isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and isinstance(node.func.value, ast.Attribute)
            and node.func.value.attr == "Secret"
            and isinstance(node.func.value.value, ast.Name)
            and node.func.value.value.id == "modal"
        ):
            found.append((node.func.attr, node))
    return found


class SelaLLMAppConfigTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = APP_PATH.read_text(encoding="utf-8")
        cls.tree = ast.parse(cls.source, filename=str(APP_PATH))
        cls.ns = _load_pure_namespace(cls.tree)
        servers = _decorator_call(cls.tree, "server")
        functions = _decorator_call(cls.tree, "function")
        assert len(servers) == 1, "expected exactly one @app.server"
        cls.server_class, cls.server_call = servers[0]
        cls.server_kwargs = _kwargs(cls.server_call)
        cls.functions = {node.name: _kwargs(call) for node, call in functions}
        cls.command = cls.ns["build_vllm_command"]("/models/pinned-snapshot")

    def value(self, node):
        """Evaluate a decorator argument against the pure constants."""
        return eval(compile(ast.Expression(node), str(APP_PATH), "eval"), dict(self.ns))

    def flag_value(self, flag):
        self.assertIn(flag, self.command)
        return self.command[self.command.index(flag) + 1]

    def test_does_not_import_modal(self):
        self.assertNotIn("modal", sys.modules, "tests must run without the modal package")

    def test_revision_pinned_to_full_sha(self):
        revision = self.ns["MODEL_REVISION"]
        self.assertRegex(revision, r"^[0-9a-f]{40}$")
        self.assertEqual(revision, PINNED_REVISION)
        self.assertEqual(self.ns["MODEL_ID"], "meta-models/Muse-Glimmer-30B")
        self.assertTrue(self.ns["snapshot_dir"]().endswith(f"/snapshots/{PINNED_REVISION}"))
        self.assertIn("models--meta-models--Muse-Glimmer-30B", self.ns["snapshot_dir"]())

    def test_vllm_and_modal_versions_pinned(self):
        self.assertRegex(self.ns["VLLM_VERSION"], r"^\d+\.\d+\.\d+$")
        self.assertEqual(self.ns["VLLM_VERSION"], "0.32.0")
        self.assertEqual(self.ns["VLLM_IMAGE"], "vllm/vllm-openai:v0.32.0")
        for floating in ("latest", "nightly", "muse-glimmer"):
            self.assertNotIn(floating, self.ns["VLLM_IMAGE"])
        requirements = [
            line.strip()
            for line in REQUIREMENTS_PATH.read_text(encoding="utf-8").splitlines()
            if line.strip() and not line.strip().startswith("#")
        ]
        self.assertEqual(requirements, ["modal==1.6.2"])

    def test_served_name_encodes_short_revision(self):
        name = self.ns["SERVED_MODEL_NAME"]
        self.assertEqual(name, PINNED_SERVED_NAME)
        self.assertIn(self.ns["MODEL_REVISION"][:7], name)
        self.assertEqual(self.flag_value("--served-model-name"), name)
        self.assertEqual(smoke_test.EXPECTED_SERVED_MODEL, name)
        self.assertEqual(smoke_test.EXPECTED_REVISION, self.ns["MODEL_REVISION"])

    def test_app_and_server_names(self):
        self.assertEqual(self.ns["APP_NAME"], "sela-llm")
        self.assertEqual(self.value(self.server_kwargs["name"]), "serve")
        app_assign = [
            node
            for node in self.tree.body
            if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == "app" for t in node.targets)
        ]
        self.assertEqual(len(app_assign), 1)
        call = app_assign[0].value
        self.assertEqual(ast.unparse(call.func), "modal.App")
        self.assertEqual([ast.unparse(arg) for arg in call.args], ["APP_NAME"])

    def test_proxy_auth_not_disabled(self):
        if "unauthenticated" in self.server_kwargs:
            node = self.server_kwargs["unauthenticated"]
            self.assertIsInstance(node, ast.Constant)
            self.assertIs(node.value, False)
        self.assertNotRegex(self.source, r"unauthenticated\s*=\s*True")
        self.assertNotRegex(self.source, r"requires_proxy_auth\s*=\s*False")

    def test_scaling_and_cost_bounds(self):
        kwargs = self.server_kwargs
        self.assertEqual(self.value(kwargs["max_containers"]), 1)
        if "min_containers" in kwargs:
            self.assertEqual(self.value(kwargs["min_containers"]), 0)
        if "buffer_containers" in kwargs:
            self.assertEqual(self.value(kwargs["buffer_containers"]), 0)
        self.assertLessEqual(self.value(kwargs["scaledown_window"]), 600)
        self.assertEqual(self.value(kwargs["scaledown_window"]), 300)
        self.assertGreaterEqual(self.value(kwargs["startup_timeout"]), 600)
        self.assertEqual(self.value(kwargs["gpu"]), "H100")
        max_num_seqs = int(self.flag_value("--max-num-seqs"))
        self.assertEqual(max_num_seqs, 8)
        self.assertEqual(self.value(kwargs["target_concurrency"]), max_num_seqs)
        self.assertEqual(self.value(kwargs["max_concurrency"]), max_num_seqs)
        cpu = self.value(kwargs["cpu"])
        memory = self.value(kwargs["memory"])
        self.assertEqual(len(cpu), 2, "cpu must be (request, limit) to cap cost")
        self.assertEqual(len(memory), 2, "memory must be (request, limit) to cap cost")
        self.assertLessEqual(cpu[0], cpu[1])
        self.assertLessEqual(memory[0], memory[1])
        download = self.functions["download_weights"]
        self.assertNotIn("gpu", download, "the weight download must not use a GPU")
        self.assertEqual(self.value(download["max_containers"]), 1)

    def test_secrets_referenced_by_name_only(self):
        calls = _modal_secret_calls(self.tree)
        self.assertTrue(calls)
        for method, call in calls:
            self.assertIn(method, ("from_name", "from_dict"), f"modal.Secret.{method} is not allowed")
            if method == "from_dict":
                # Only the documented empty placeholder (no values) is allowed.
                self.assertTrue(
                    call.args and isinstance(call.args[0], ast.Dict) and not call.args[0].keys,
                    "modal.Secret.from_dict may only be used with an empty dict",
                )
            else:
                self.assertTrue(call.args and isinstance(call.args[0], ast.Name | ast.Constant))
                self.assertIn("required_keys", _kwargs(call))
        names = [self.value(call.args[0]) for method, call in calls if method == "from_name" and isinstance(call.args[0], ast.Name) and call.args[0].id in self.ns]
        self.assertIn("sela-llm-api", names)
        api = [call for method, call in calls if method == "from_name" and ast.unparse(call.args[0]) == "API_KEY_SECRET_NAME"]
        self.assertEqual(len(api), 1)
        self.assertEqual(self.value(_kwargs(api[0])["required_keys"]), ["VLLM_API_KEY"])
        self.assertEqual(ast.unparse(self.server_kwargs["secrets"]), "[api_key_secret]")

    def test_api_key_never_in_argv_or_plain_env(self):
        self.assertNotIn("--api-key", self.command)
        self.assertEqual(self.ns["API_KEY_ENV"], "VLLM_API_KEY")
        self.assertNotIn("VLLM_API_KEY", self.ns["SERVER_ENV"])
        self.assertGreaterEqual(self.ns["MIN_API_KEY_LENGTH"], 32)

    def test_no_token_like_strings_or_emails_in_new_files(self):
        paths = [APP_PATH, SMOKE_PATH, README_PATH, NOTES_PATH, REQUIREMENTS_PATH, WORKFLOW_PATH, Path(__file__)]
        for path in paths:
            text = path.read_text(encoding="utf-8")
            for pattern in TOKEN_PATTERNS:
                self.assertIsNone(pattern.search(text), f"token-like string in {path.name}")
            self.assertIsNone(EMAIL_PATTERN.search(text), f"email address in {path.name}")
        for node in ast.walk(self.tree):
            if isinstance(node, ast.Constant) and isinstance(node.value, str):
                self.assertNotRegex(node.value, r"^(?:wk|ws|ak|as)-")
                self.assertFalse(node.value.startswith("hf_"))

    def test_request_logging_disabled(self):
        self.assertIn("--no-enable-log-requests", self.command)
        self.assertNotIn("--enable-log-requests", self.command)
        self.assertNotIn("--enable-log-outputs", self.command)
        self.assertNotIn("--disable-log-requests", self.command, "flag removed in vLLM 0.32")
        self.assertIn("--disable-uvicorn-access-log", self.command)
        self.assertNotEqual(self.ns["SERVER_ENV"]["VLLM_LOGGING_LEVEL"].upper(), "DEBUG")

    def test_max_model_len_bounded(self):
        max_model_len = int(self.flag_value("--max-model-len"))
        self.assertEqual(max_model_len, 16384)
        self.assertLessEqual(max_model_len, 32768)
        utilization = float(self.flag_value("--gpu-memory-utilization"))
        self.assertGreater(utilization, 0.5)
        self.assertLessEqual(utilization, 0.95)

    def test_model_specific_flags(self):
        self.assertEqual(self.command[:3], ["vllm", "serve", "/models/pinned-snapshot"])
        self.assertEqual(self.flag_value("--reasoning-parser"), "muse_glimmer")
        self.assertEqual(self.flag_value("--tool-call-parser"), "muse_glimmer")
        self.assertEqual(self.flag_value("--generation-config"), "auto")
        self.assertEqual(self.flag_value("--host"), "0.0.0.0")
        self.assertEqual(self.flag_value("--port"), str(self.value(self.server_kwargs["port"])))
        self.assertIn("--language-model-only", self.command)
        self.assertEqual(json.loads(self.flag_value("--default-chat-template-kwargs")), {"reasoning_strength": "low"})
        self.assertNotIn("--chat-template", self.command, "Meta: use the checkpoint's template")
        self.assertNotIn("--trust-remote-code", self.command)

    def test_server_runs_offline_from_read_only_weights(self):
        env = self.ns["SERVER_ENV"]
        self.assertEqual(env["HF_HUB_OFFLINE"], "1")
        self.assertNotEqual(env["HF_HOME"], self.ns["HF_CACHE_DIR"])
        volumes = self.server_kwargs["volumes"]
        self.assertIsInstance(volumes, ast.Dict)
        mounts = {ast.unparse(k): v for k, v in zip(volumes.keys, volumes.values)}
        weights = mounts["HF_CACHE_DIR"]
        self.assertIsInstance(weights, ast.Call)
        self.assertEqual(weights.func.attr, "with_mount_options")
        self.assertIs(_kwargs(weights)["read_only"].value, True)
        self.assertIn("VLLM_CACHE_DIR", mounts, "compile cache Volume must be mounted")

    def test_expected_files_manifest(self):
        files = self.ns["EXPECTED_FILES"]
        self.assertEqual(sum(size for size, _, _ in files.values()), 59581830875)
        for shard in ("model-00001-of-00002.safetensors", "model-00002-of-00002.safetensors"):
            size, algorithm, digest = files[shard]
            self.assertEqual(algorithm, "sha256")
            self.assertRegex(digest, r"^[0-9a-f]{64}$")
        for name in ("config.json", "generation_config.json", "chat_template.jinja", "tokenizer_config.json"):
            self.assertEqual(files[name][1], "git-sha1")
            self.assertRegex(files[name][2], r"^[0-9a-f]{40}$")

    def test_git_blob_sha1_matches_git(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "hello.txt")
            with open(path, "wb") as handle:
                handle.write(b"hello\n")
            # `printf 'hello\n' | git hash-object --stdin`
            self.assertEqual(self.ns["git_blob_sha1"](path), "ce013625030ba8dba906f756967f9e9ca394464a")

    def test_verify_snapshot_shallow_and_deep(self):
        verify = self.ns["verify_snapshot"]
        with tempfile.TemporaryDirectory() as tmp:
            blobs = {"small.json": b'{"a": 1}\n', "sub/weights.bin": b"\x00" * 1000}
            for rel, data in blobs.items():
                os.makedirs(os.path.dirname(os.path.join(tmp, rel)), exist_ok=True)
                with open(os.path.join(tmp, rel), "wb") as handle:
                    handle.write(data)
            small = blobs["small.json"]
            expected = {
                "small.json": (len(small), "git-sha1", hashlib.sha1(b"blob %d\0" % len(small) + small).hexdigest()),
                "sub/weights.bin": (1000, "sha256", hashlib.sha256(blobs["sub/weights.bin"]).hexdigest()),
            }
            self.assertEqual(verify(tmp, deep=False, expected=expected), [])
            self.assertEqual(verify(tmp, deep=True, expected=expected), [])

            wrong_digest = dict(expected, **{"sub/weights.bin": (1000, "sha256", "0" * 64)})
            self.assertEqual(verify(tmp, deep=False, expected=wrong_digest), [])
            self.assertEqual(verify(tmp, deep=True, expected=wrong_digest), ["sha256 mismatch: sub/weights.bin"])

            wrong_size = dict(expected, **{"small.json": (3, "git-sha1", expected["small.json"][2])})
            self.assertTrue(verify(tmp, deep=False, expected=wrong_size)[0].startswith("size mismatch: small.json"))

            missing = dict(expected, **{"absent.txt": (1, "git-sha1", "0" * 40)})
            self.assertEqual(verify(tmp, deep=False, expected=missing), ["missing: absent.txt"])

    def test_wait_until_healthy_fails_fast_when_vllm_exits(self):
        class Exited:
            def poll(self):
                return 3

        sleeps = []
        with self.assertRaisesRegex(RuntimeError, "exited with code 3"):
            self.ns["wait_until_healthy"](Exited(), 60, "http://127.0.0.1:9/health", sleep=sleeps.append)
        self.assertEqual(sleeps, [])

    def test_wait_until_healthy_against_local_server(self):
        class Running:
            def poll(self):
                return None

        with _FakeHealthServer(statuses=[503, 503, 200]) as server:
            sleeps = []
            waited = self.ns["wait_until_healthy"](
                Running(), 60, f"{server.url}/health", poll_interval_s=0.01, sleep=sleeps.append
            )
            self.assertGreaterEqual(waited, 0)
            self.assertEqual(len(sleeps), 2)

        ticks = iter(range(0, 1000, 10))
        with _FakeHealthServer(statuses=[503] * 100) as server:
            with self.assertRaises(TimeoutError):
                self.ns["wait_until_healthy"](
                    Running(), 30, f"{server.url}/health", clock=lambda: next(ticks), sleep=lambda s: None
                )


class _FakeHealthServer:
    def __init__(self, statuses):
        self.statuses = list(statuses)

    def __enter__(self):
        statuses = self.statuses

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                status = statuses.pop(0) if statuses else 200
                self.send_response(status)
                self.send_header("Content-Length", "0")
                self.end_headers()

            def log_message(self, *args):
                pass

        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.httpd.server_address[1]}"
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()
        return self

    def __exit__(self, *exc):
        self.httpd.shutdown()
        self.httpd.server_close()


# ---------------------------------------------------------------------------
# Workflow
# ---------------------------------------------------------------------------


class ModalDeployWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = WORKFLOW_PATH.read_text(encoding="utf-8")

    def test_manual_dispatch_only(self):
        self.assertRegex(self.text, r"(?m)^on:\n  workflow_dispatch:\n")
        for trigger in ("push", "pull_request", "pull_request_target", "schedule", "workflow_run", "workflow_call"):
            self.assertNotRegex(self.text, rf"(?m)^\s+{trigger}:", f"must not run on {trigger}")

    def test_inputs(self):
        self.assertRegex(self.text, r"(?s)environment:.*type: choice.*- dev.*- staging")
        self.assertRegex(self.text, r"(?s)run_smoke:.*type: boolean.*default: false")

    def test_actions_pinned_by_major_version_like_quality(self):
        uses = re.findall(r"uses:\s*(\S+)", self.text)
        self.assertTrue(uses)
        for ref in uses:
            self.assertRegex(ref, r"^actions/[a-z-]+@v\d+$")
        quality_checkout = re.search(r"actions/checkout@(v\d+)", QUALITY_WORKFLOW_PATH.read_text(encoding="utf-8"))
        self.assertIn(f"actions/checkout@{quality_checkout.group(1)}", uses)

    def test_secrets_only_via_env_and_never_echoed(self):
        for line in self.text.splitlines():
            if "secrets." in line and not line.strip().startswith("#"):
                self.assertRegex(line.strip(), r"^[A-Z_]+: \$\{\{ secrets\.[A-Z_]+ \}\}$", line)
        for secret_var in ("MODAL_TOKEN_ID", "MODAL_TOKEN_SECRET", "SELA_LLM_API_KEY", "SELA_MODAL_KEY", "SELA_MODAL_SECRET"):
            self.assertNotRegex(self.text, rf"(?m)^\s*(echo|printf).*\${secret_var}\b")
        self.assertNotRegex(self.text, r"(?m)^\s*set -[a-z]*x")
        self.assertRegex(self.text, r"(?m)^permissions:\n  contents: read$")

    def test_deploys_selected_environment_with_recreate(self):
        self.assertIn("MODAL_ENVIRONMENT: ${{ inputs.environment }}", self.text)
        self.assertIn('modal deploy --env "$MODAL_ENVIRONMENT" --strategy recreate modal/sela_llm.py', self.text)
        self.assertIn("python modal/smoke_test.py", self.text)
        self.assertRegex(self.text, r"if: \$\{\{ inputs\.run_smoke \}\}")


# ---------------------------------------------------------------------------
# smoke_test.py pure functions
# ---------------------------------------------------------------------------


class SmokePureFunctionTest(unittest.TestCase):
    def test_stdlib_only(self):
        tree = ast.parse(SMOKE_PATH.read_text(encoding="utf-8"))
        modules = set()
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                modules.update(alias.name.split(".")[0] for alias in node.names)
            elif isinstance(node, ast.ImportFrom) and node.level == 0:
                modules.add((node.module or "").split(".")[0])
        modules.discard("__future__")
        self.assertTrue(modules <= set(sys.stdlib_module_names), modules - set(sys.stdlib_module_names))

    def test_backoff_schedule(self):
        self.assertEqual(list(smoke_test.backoff_delays(100)), [2, 4, 8, 16, 30, 30, 10])
        self.assertEqual(sum(smoke_test.backoff_delays(1200)), 1200)
        self.assertTrue(all(delay <= 30 for delay in smoke_test.backoff_delays(1200)))
        self.assertEqual(list(smoke_test.backoff_delays(0)), [])
        self.assertEqual(list(smoke_test.backoff_delays(5, initial_s=10)), [5])
        self.assertEqual(list(smoke_test.backoff_delays(7, initial_s=1, factor=3, max_delay_s=4)), [1, 3, 3])

    def test_schema_validation(self):
        schema = smoke_test.SMOKE_SCHEMA
        good = {"language": "id", "main_concern": "susah tidur", "exact_quote": "susah tidur", "mentions_sleep": True}
        self.assertEqual(smoke_test.validate_json_schema(good, schema), [])
        self.assertEqual(smoke_test.validate_json_schema([], schema), ["$: expected type object"])
        errors = smoke_test.validate_json_schema(
            {"language": "fr", "main_concern": "", "mentions_sleep": "yes", "extra": 1}, schema
        )
        self.assertIn("$: missing required property 'exact_quote'", errors)
        self.assertIn("$.language: value not in enum", errors)
        self.assertIn("$.main_concern: shorter than minLength 1", errors)
        self.assertIn("$.mentions_sleep: expected type boolean", errors)
        self.assertIn("$: unexpected property 'extra'", errors)
        self.assertFalse(any("fr" in error or "yes" in error for error in errors), "errors must not echo values")
        nested = {"type": "array", "items": {"type": "integer", "maximum": 3}, "maxItems": 2}
        self.assertEqual(
            smoke_test.validate_json_schema([1, 5, True], nested),
            ["$: more than maxItems", "$[1]: above maximum", "$[2]: expected type integer"],
        )

    def test_stream_accumulator(self):
        ticks = iter([0.0, 1.5, 2.0, 3.0, 4.0, 5.0, 6.0])
        acc = smoke_test.StreamAccumulator(clock=lambda: next(ticks))
        lines = [
            ": keep-alive",
            'data: {"model": "m", "choices": [{"index": 0, "delta": {"role": "assistant", "content": ""}}]}',
            "",
            'data: {"model": "m", "choices": [{"index": 0, "delta": {"reasoning": "think "}}]}',
            "",
            'data: {"model": "m", "choices": [{"index": 0, "delta": {"reasoning_content": "more"}}]}',
            "",
            'data: {"model": "m", "choices": [{"index": 0, "delta": {"content": "{\\"a\\""}}]}',
            "",
            'data: {"model": "m", "choices": [{"index": 0, "delta": {"content": ": 1}"}, "finish_reason": "stop"}]}',
            "",
            "data: not json",
            "",
            'data: {"model": "m", "choices": [], "usage": {"prompt_tokens": 5, "completion_tokens": 9}}',
            "",
            "data: [DONE]",
            "",
        ]
        for line in lines:
            acc.feed_line(line + "\n")
        self.assertTrue(acc.done)
        self.assertEqual(acc.content, '{"a": 1}')
        self.assertEqual(acc.content_deltas, 2)
        self.assertEqual(acc.reasoning_deltas, 2)
        self.assertEqual(acc.reasoning_fields, {"reasoning", "reasoning_content"})
        self.assertEqual(acc.reasoning_chars, len("think more"))
        # The clock is read only for the first reasoning and first content delta.
        self.assertEqual(acc.first_reasoning_s, 1.5)
        self.assertEqual(acc.first_content_s, 2.0)
        self.assertEqual(acc.finish_reason, "stop")
        self.assertEqual(acc.usage["completion_tokens"], 9)
        self.assertEqual(acc.models, {"m"})
        self.assertEqual(acc.malformed_events, 1)
        self.assertFalse(hasattr(acc, "reasoning_text"), "reasoning text must not be kept")

    def test_build_report_redacts_and_hides_output(self):
        checks = {"a": {"ok": True, "error": f"server said {FAKE_API_KEY} and {FAKE_MODAL_SECRET}"}}
        report = smoke_test.build_report(
            checks, {"host": "h"}, "t0", "t1", secret_values=[FAKE_API_KEY, FAKE_MODAL_SECRET], content="generated"
        )
        dumped = json.dumps(report)
        self.assertNotIn(FAKE_API_KEY, dumped)
        self.assertNotIn(FAKE_MODAL_SECRET, dumped)
        self.assertIn("[REDACTED]", dumped)
        self.assertNotIn("generated", dumped)
        self.assertTrue(report["ok"])
        shown = smoke_test.build_report(checks, {}, "t0", "t1", content="generated", show_output=True)
        self.assertEqual(shown["output"], {"content": "generated"})
        self.assertFalse(smoke_test.build_report({"a": {"ok": True}, "b": {"ok": False}}, {}, "t0", "t1")["ok"])
        self.assertFalse(smoke_test.build_report({}, {}, "t0", "t1")["ok"])

    def test_parse_base_url(self):
        self.assertEqual(smoke_test.parse_base_url("https://x--sela-llm-serve.us-east.modal.direct/"), "https://x--sela-llm-serve.us-east.modal.direct")
        self.assertEqual(smoke_test.parse_base_url("http://127.0.0.1:8080"), "http://127.0.0.1:8080")
        for bad in ("http://example.com", "ftp://x", "https://user:pw@127.0.0.1", "https://x.example/?k=v", "not a url"):
            with self.assertRaises(smoke_test.ConfigError):
                smoke_test.parse_base_url(bad)

    def test_chat_payload(self):
        payload = smoke_test.build_chat_payload("served", 512, "medium", {"top_k": 64})
        for key in ("temperature", "top_p", "reasoning_effort"):
            self.assertNotIn(key, payload)
        self.assertEqual(payload["top_k"], 64)
        self.assertEqual(payload["chat_template_kwargs"], {"reasoning_strength": "medium"})
        self.assertEqual(payload["response_format"]["type"], "json_schema")
        self.assertEqual(payload["response_format"]["json_schema"]["schema"], smoke_test.SMOKE_SCHEMA)
        self.assertTrue(payload["stream"])
        self.assertEqual(payload["stream_options"], {"include_usage": True})
        self.assertEqual(payload["model"], "served")

    def test_network_failure_classification(self):
        import socket
        import urllib.error

        self.assertTrue(smoke_test._network_failure(urllib.error.URLError(socket.gaierror(-2, "Name or service not known"))).fatal)
        self.assertFalse(smoke_test._network_failure(urllib.error.URLError(ConnectionRefusedError(111, "refused"))).fatal)
        self.assertFalse(smoke_test._network_failure(TimeoutError("timed out")).fatal)


# ---------------------------------------------------------------------------
# smoke_test.py against a local fake of Modal's proxy + vLLM
# ---------------------------------------------------------------------------


GOOD_CONTENT = json.dumps(
    {
        "language": "id",
        "main_concern": "susah tidur karena tugas kuliah",
        "exact_quote": "saya susah tidur karena tugas kuliah menumpuk",
        "mentions_sleep": True,
    }
)


class FakeModalVLLM:
    """Imitates Modal proxy auth, cold start (503) and vLLM's API."""

    def __init__(self, cold_503=0, public=False, chat_mode="ok", redirect=None):
        self.cold_503 = cold_503
        self.public = public
        self.chat_mode = chat_mode
        # redirect: {"code": 3xx, "location": url, "methods": {"GET", "POST"}}
        # answers matching requests with a redirect before any auth check.
        self.redirect = redirect
        self.requests = []
        self.lock = threading.Lock()

    def __enter__(self):
        fake = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def _record(self, body=None):
                with fake.lock:
                    fake.requests.append(
                        {
                            "method": self.command,
                            "path": self.path,
                            "modal_key": self.headers.get("Modal-Key"),
                            "modal_secret": self.headers.get("Modal-Secret"),
                            "authorization": self.headers.get("Authorization"),
                            "body": body,
                        }
                    )

            def _send(self, status, payload, content_type="application/json"):
                data = payload if isinstance(payload, bytes) else json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("Content-Type", content_type)
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def _redirected(self):
                if fake.redirect and self.command in fake.redirect["methods"]:
                    self.send_response(fake.redirect["code"])
                    self.send_header("Location", fake.redirect["location"] + self.path)
                    self.send_header("Content-Length", "0")
                    self.end_headers()
                    return True
                return False

            def _gate(self):
                """Modal proxy, then cold start, then vLLM API key. True = pass."""
                if not fake.public and (
                    self.headers.get("Modal-Key") != FAKE_MODAL_KEY or self.headers.get("Modal-Secret") != FAKE_MODAL_SECRET
                ):
                    self._send(401, b"modal-http: missing credentials for proxy authorization", "text/plain")
                    return False
                with fake.lock:
                    cold = fake.cold_503 > 0
                    if cold:
                        fake.cold_503 -= 1
                if cold:
                    self._send(503, b"no containers ready", "text/plain")
                    return False
                if self.path.startswith("/v1") and self.headers.get("Authorization") != f"Bearer {FAKE_API_KEY}":
                    self._send(401, {"error": "Unauthorized"})
                    return False
                return True

            def do_GET(self):
                self._record()
                if self._redirected() or not self._gate():
                    return
                if self.path == "/health":
                    self._send(200, b"", "text/plain")
                elif self.path == "/v1/models":
                    root = f"/root/.cache/huggingface/hub/models--meta-models--Muse-Glimmer-30B/snapshots/{PINNED_REVISION}"
                    self._send(
                        200,
                        {
                            "object": "list",
                            "data": [{"id": PINNED_SERVED_NAME, "object": "model", "root": root, "max_model_len": 16384}],
                        },
                    )
                else:
                    self._send(404, {"error": "not found"})

            def do_POST(self):
                length = int(self.headers.get("Content-Length", "0"))
                body = json.loads(self.rfile.read(length) or b"{}")
                self._record(body)
                if self._redirected() or not self._gate():
                    return
                content = GOOD_CONTENT if fake.chat_mode != "invalid_json" else "Tentu! " + GOOD_CONTENT
                reasoning_field = "reasoning_content" if fake.chat_mode == "reasoning_content" else "reasoning"

                def chunk(delta, finish=None):
                    return {
                        "id": "chatcmpl-test",
                        "object": "chat.completion.chunk",
                        "created": 0,
                        "model": PINNED_SERVED_NAME,
                        "choices": [{"index": 0, "delta": delta, "finish_reason": finish}],
                    }

                events = [chunk({"role": "assistant", "content": ""})]
                events += [chunk({reasoning_field: f"step {i}. "}) for i in range(3)]
                events += [chunk({"content": content[i : i + 40]}) for i in range(0, len(content), 40)]
                events.append(chunk({}, finish="stop"))
                events.append(
                    {
                        "id": "chatcmpl-test",
                        "object": "chat.completion.chunk",
                        "created": 0,
                        "model": PINNED_SERVED_NAME,
                        "choices": [],
                        "usage": {"prompt_tokens": 95, "completion_tokens": 61, "total_tokens": 156},
                    }
                )
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Cache-Control", "no-cache")
                self.end_headers()
                self.wfile.write(b": keep-alive\n\n")
                for event in events:
                    self.wfile.write(b"data: " + json.dumps(event).encode() + b"\n\n")
                    self.wfile.flush()
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()

        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.base_url = f"http://127.0.0.1:{self.httpd.server_address[1]}"
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()
        return self

    def __exit__(self, *exc):
        self.httpd.shutdown()
        self.httpd.server_close()

    def config(self, **overrides):
        config = {
            "base_url": self.base_url,
            "api_key": FAKE_API_KEY,
            "modal_key": FAKE_MODAL_KEY,
            "modal_secret": FAKE_MODAL_SECRET,
            "expected_model": PINNED_SERVED_NAME,
            "expected_revision": PINNED_REVISION,
            "cold_start_budget_s": 60.0,
            "request_timeout_s": 10.0,
            "chat_deadline_s": 30.0,
            "max_tokens": 512,
            "reasoning_strength": "low",
            "extra_body": None,
            "show_output": False,
            "skip_chat": False,
        }
        config.update(overrides)
        return config

    def environ(self):
        return {
            "SELA_LLM_BASE_URL": self.base_url,
            "SELA_LLM_API_KEY": FAKE_API_KEY,
            "SELA_MODAL_KEY": FAKE_MODAL_KEY,
            "SELA_MODAL_SECRET": FAKE_MODAL_SECRET,
        }


class SmokeAgainstFakeServerTest(unittest.TestCase):
    def run_quietly(self, fake, sleeps=None, **overrides):
        sleeps = [] if sleeps is None else sleeps
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            report, code = smoke_test.run_smoke(fake.config(**overrides), sleep=sleeps.append)
        return report, code, stderr.getvalue()

    def assert_no_secrets(self, *texts):
        for text in texts:
            for secret in (FAKE_API_KEY, FAKE_MODAL_KEY, FAKE_MODAL_SECRET):
                self.assertNotIn(secret, text)

    def test_cold_start_then_success(self):
        with FakeModalVLLM(cold_503=2) as fake:
            sleeps = []
            report, code, stderr = self.run_quietly(fake, sleeps)
            requests = list(fake.requests)
        checks = report["checks"]
        self.assertEqual(code, 0, json.dumps(report, indent=2))
        self.assertTrue(report["ok"])
        self.assertEqual(
            checks["unauthenticated_rejected"], {"ok": True, "status": 401, "path": "/health", "error": None}
        )
        self.assertEqual(checks["cold_start"]["attempts"], 3)
        self.assertEqual(checks["cold_start"]["status_counts"], {"503": 2, "200": 1})
        self.assertEqual(sleeps, [2, 4])
        self.assertIsInstance(checks["cold_start"]["cold_start_seconds"], float)
        self.assertTrue(checks["models"]["ok"])
        self.assertTrue(checks["models"]["root_contains_revision"])
        self.assertEqual(checks["vllm_key_enforced"]["status"], 401)
        chat = checks["chat_stream"]
        self.assertTrue(chat["schema_valid"])
        self.assertTrue(chat["reasoning_present"])
        self.assertEqual(chat["reasoning_fields"], ["reasoning"])
        self.assertEqual(chat["reasoning_deltas"], 3)
        self.assertEqual(chat["completion_tokens"], 61)
        self.assertEqual(chat["finish_reason"], "stop")
        self.assertTrue(chat["model_matches"])
        self.assertIsNotNone(chat["ttfb_seconds"])
        self.assertGreaterEqual(chat["total_seconds"], chat["ttfb_seconds"])
        self.assertEqual(chat["quality_signals"], {"language_is_id": True, "mentions_sleep": True, "quote_is_verbatim": True})

        dumped = json.dumps(report)
        self.assert_no_secrets(dumped, stderr)
        self.assertNotIn("output", report)
        self.assertNotIn("menumpuk", dumped, "generated text must not appear without --show-output")
        self.assertNotIn("step 0", dumped, "reasoning text must never appear")

        unauthenticated = requests[0]
        self.assertEqual(unauthenticated["path"], "/health")
        self.assertIsNone(unauthenticated["modal_key"])
        self.assertIsNone(unauthenticated["authorization"])
        chat_requests = [r for r in requests if r["method"] == "POST"]
        self.assertEqual(len(chat_requests), 1)
        sent = chat_requests[0]
        self.assertEqual(sent["authorization"], f"Bearer {FAKE_API_KEY}")
        self.assertEqual(sent["modal_key"], FAKE_MODAL_KEY)
        body = sent["body"]
        self.assertNotIn("temperature", body)
        self.assertEqual(body["chat_template_kwargs"], {"reasoning_strength": "low"})
        self.assertEqual(body["response_format"]["type"], "json_schema")
        self.assertEqual(body["messages"][1]["content"], smoke_test.SYNTHETIC_TEXT)

    def test_show_output_includes_content_only(self):
        with FakeModalVLLM() as fake:
            report, code, _ = self.run_quietly(fake, show_output=True)
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(report["output"]["content"]), json.loads(GOOD_CONTENT))
        self.assertNotIn("step 0", json.dumps(report))

    def test_cold_start_budget_exhausted(self):
        with FakeModalVLLM(cold_503=10**6) as fake:
            sleeps = []
            report, code, _ = self.run_quietly(fake, sleeps, cold_start_budget_s=10.0)
        self.assertEqual(code, 1)
        self.assertFalse(report["ok"])
        cold = report["checks"]["cold_start"]
        self.assertFalse(cold["ok"])
        self.assertEqual(cold["reason"], "budget exhausted")
        self.assertLessEqual(sum(sleeps), 10)
        self.assertEqual(report["checks"]["chat_stream"], {"ok": False, "skipped": "server never became ready"})

    def test_wrong_api_key_is_not_retried(self):
        with FakeModalVLLM() as fake:
            sleeps = []
            report, code, _ = self.run_quietly(fake, sleeps, api_key="test-other-key-" + "9" * 32)
        self.assertEqual(code, 1)
        self.assertEqual(report["checks"]["cold_start"]["reason"], "non-retryable status")
        self.assertEqual(report["checks"]["cold_start"]["last_status"], 401)
        self.assertEqual(sleeps, [])

    def test_public_endpoint_fails_auth_check(self):
        # Proxy auth off: vLLM still answers 401 on /v1 without its key, which
        # must not mask the public endpoint. /health exposes it.
        with FakeModalVLLM(public=True) as fake:
            report, code, _ = self.run_quietly(fake)
        self.assertEqual(code, 1)
        self.assertFalse(report["checks"]["unauthenticated_rejected"]["ok"])
        self.assertEqual(report["checks"]["unauthenticated_rejected"]["status"], 200)

    def test_invalid_json_content_fails(self):
        with FakeModalVLLM(chat_mode="invalid_json") as fake:
            report, code, _ = self.run_quietly(fake)
        chat = report["checks"]["chat_stream"]
        self.assertEqual(code, 1)
        self.assertFalse(chat["json_parse_ok"])
        self.assertFalse(chat["schema_valid"])
        self.assertEqual(chat["schema_errors"], ["content is not valid JSON"])

    def test_reasoning_content_field_variant(self):
        with FakeModalVLLM(chat_mode="reasoning_content") as fake:
            report, code, _ = self.run_quietly(fake)
        self.assertEqual(code, 0)
        self.assertEqual(report["checks"]["chat_stream"]["reasoning_fields"], ["reasoning_content"])

    def test_main_reads_env_and_prints_safe_json(self):
        with FakeModalVLLM() as fake:
            stdout, stderr = io.StringIO(), io.StringIO()
            with contextlib.redirect_stderr(stderr):
                code = smoke_test.main(["--cold-start-budget", "5"], environ=fake.environ(), stdout=stdout)
        self.assertEqual(code, 0, stdout.getvalue())
        report = json.loads(stdout.getvalue())
        self.assertTrue(report["ok"])
        self.assertEqual(report["target"]["host"], "127.0.0.1")
        self.assert_no_secrets(stdout.getvalue(), stderr.getvalue())

    def test_main_reports_missing_env_without_values(self):
        stdout = io.StringIO()
        code = smoke_test.main([], environ={"SELA_LLM_API_KEY": FAKE_API_KEY}, stdout=stdout)
        self.assertEqual(code, 2)
        payload = json.loads(stdout.getvalue())
        self.assertFalse(payload["ok"])
        self.assertIn("SELA_LLM_BASE_URL", payload["config_error"])
        self.assertIn("SELA_MODAL_SECRET", payload["config_error"])
        self.assertNotIn("SELA_LLM_API_KEY", payload["config_error"], "only missing names are listed")
        self.assert_no_secrets(stdout.getvalue())

    def test_extra_body_cannot_disable_streaming(self):
        with FakeModalVLLM() as fake:
            code = smoke_test.main(["--extra-body", '{"stream": false}'], environ=fake.environ(), stdout=io.StringIO())
        self.assertEqual(code, 2)


# ---------------------------------------------------------------------------
# Redirects must never carry credentials to another host
# ---------------------------------------------------------------------------

CREDENTIAL_HEADERS = ("Modal-Key", "Modal-Secret", "Authorization")
REDIRECT_CODES = (301, 302, 303, 307, 308)


class _CredentialSink:
    """A different host:port that records every request it receives."""

    def __enter__(self):
        sink = self
        self.requests = []

        class Handler(BaseHTTPRequestHandler):
            def _answer(self):
                sink.requests.append(
                    {"method": self.command, "path": self.path, **{h: self.headers.get(h) for h in CREDENTIAL_HEADERS}}
                )
                data = json.dumps({"object": "list", "data": [{"id": PINNED_SERVED_NAME}]}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            do_GET = do_POST = _answer

            def log_message(self, *args):
                pass

        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        # "localhost" instead of 127.0.0.1: a different host name and port
        # from the fake Modal server, like a redirect to a foreign host.
        self.url = f"http://localhost:{self.httpd.server_address[1]}"
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()
        return self

    def __exit__(self, *exc):
        self.httpd.shutdown()
        self.httpd.server_close()


class SmokeRedirectTest(unittest.TestCase):
    def run_quietly(self, fake, sleeps):
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            report, code = smoke_test.run_smoke(fake.config(), sleep=sleeps.append)
        return report, code, stderr.getvalue()

    def assert_no_secrets(self, *texts):
        for text in texts:
            for secret in (FAKE_API_KEY, FAKE_MODAL_KEY, FAKE_MODAL_SECRET):
                self.assertNotIn(secret, text)

    def test_every_opener_replaces_the_stock_redirect_handler(self):
        import urllib.request

        for base_url in ("http://127.0.0.1:9", "https://example-dev--sela-llm-serve.us-east.modal.direct"):
            with self.subTest(base_url=base_url):
                client = smoke_test.SmokeClient(base_url, FAKE_API_KEY, FAKE_MODAL_KEY, FAKE_MODAL_SECRET, 1)
                redirect_handlers = [
                    h for h in client._opener.handlers if isinstance(h, urllib.request.HTTPRedirectHandler)
                ]
                self.assertEqual(len(redirect_handlers), 1)
                self.assertIsInstance(redirect_handlers[0], smoke_test.NoRedirectHandler)

    def test_handler_refuses_https_to_http_downgrade_and_cross_host(self):
        import email.message
        import urllib.error
        import urllib.request

        handler = smoke_test.NoRedirectHandler()
        for method in ("GET", "POST"):
            for code in REDIRECT_CODES:
                for new_url in ("http://downgrade.example/v1/models", "https://other-host.example/v1/models"):
                    with self.subTest(method=method, code=code, new_url=new_url):
                        request = urllib.request.Request(
                            "https://sela.example/v1/models", data=b"{}" if method == "POST" else None, method=method
                        )
                        with self.assertRaises(urllib.error.HTTPError) as raised:
                            handler.redirect_request(request, io.BytesIO(b""), code, "Found", email.message.Message(), new_url)
                        self.assertEqual(raised.exception.code, code)
                        raised.exception.close()

    def test_credentials_are_unredirected_headers(self):
        client = smoke_test.SmokeClient("http://127.0.0.1:9", FAKE_API_KEY, FAKE_MODAL_KEY, FAKE_MODAL_SECRET, 1)
        wanted = {name.lower() for name in CREDENTIAL_HEADERS}
        for method, auth in (("GET", "full"), ("GET", "proxy_wrong_key"), ("POST", "full")):
            with self.subTest(method=method, auth=auth):
                request = client._request(method, "/v1/models", auth, payload={} if method == "POST" else None)
                self.assertEqual({k.lower() for k in request.headers} & wanted, set(), "redirectable headers")
                self.assertEqual({k.lower() for k in request.unredirected_hdrs} & wanted, wanted)
        none = client._request("GET", "/health", "none")
        self.assertEqual({k.lower() for k in none.unredirected_hdrs} & wanted, set())

    def test_redirects_are_not_followed_and_fail_the_report(self):
        for code in REDIRECT_CODES:
            with self.subTest(code=code), _CredentialSink() as sink:
                redirect = {"code": code, "location": sink.url, "methods": {"GET", "POST"}}
                with FakeModalVLLM(redirect=redirect) as fake:
                    sleeps = []
                    report, exit_code, stderr = self.run_quietly(fake, sleeps)
                    origin_requests = list(fake.requests)
                self.assertEqual(sink.requests, [], "the redirect target must receive nothing")
                self.assertEqual(exit_code, 1)
                self.assertFalse(report["ok"])
                unauthenticated = report["checks"]["unauthenticated_rejected"]
                self.assertFalse(unauthenticated["ok"])
                self.assertEqual(unauthenticated["status"], code)
                self.assertIn(f"redirect not followed (HTTP {code} to http://localhost)", unauthenticated["error"])
                cold = report["checks"]["cold_start"]
                self.assertFalse(cold["ok"])
                self.assertEqual(cold["last_status"], code)
                self.assertEqual(cold["reason"], "non-retryable status")
                self.assertIn("redirect not followed", cold["error"])
                self.assertEqual(sleeps, [], "a redirect is not a cold start")
                self.assertEqual([r["path"] for r in origin_requests], ["/health", "/v1/models"])
                self.assertNotIn("/health", unauthenticated["error"], "redirect paths are not reported")
                self.assertNotIn("/v1/models", cold["error"], "redirect paths are not reported")
                self.assert_no_secrets(json.dumps(report), stderr)

    def test_chat_redirect_is_not_followed(self):
        # 302/303 on POST is exactly what urllib's stock handler follows as a
        # GET with every header copied; 307/308 it refuses for POST anyway.
        for code in REDIRECT_CODES:
            with self.subTest(code=code), _CredentialSink() as sink:
                with FakeModalVLLM(redirect={"code": code, "location": sink.url, "methods": {"POST"}}) as fake:
                    sleeps = []
                    report, exit_code, stderr = self.run_quietly(fake, sleeps)
                self.assertEqual(sink.requests, [])
                self.assertEqual(exit_code, 1)
                checks = report["checks"]
                for name in ("unauthenticated_rejected", "cold_start", "models", "vllm_key_enforced"):
                    self.assertTrue(checks[name]["ok"], name)
                chat = checks["chat_stream"]
                self.assertEqual(
                    (chat["ok"], chat["status"], chat["attempts"]), (False, code, 1), json.dumps(chat)
                )
                self.assertIn("redirect not followed", chat["error"])
                self.assertEqual(sleeps, [])
                self.assert_no_secrets(json.dumps(report), stderr)

    def test_sink_detects_leaks_and_unredirected_headers_survive_a_following_opener(self):
        """Defence in depth, and a control that the sink would see a leak."""
        import urllib.request

        following = urllib.request.build_opener(urllib.request.ProxyHandler({}))  # stock redirect handler
        with _CredentialSink() as sink:
            with FakeModalVLLM(redirect={"code": 302, "location": sink.url, "methods": {"GET"}}) as fake:
                leaky = urllib.request.Request(fake.base_url + "/v1/models", headers={"Modal-Secret": FAKE_MODAL_SECRET})
                with following.open(leaky, timeout=5) as response:
                    self.assertEqual(response.status, 200)
                client = smoke_test.SmokeClient(fake.base_url, FAKE_API_KEY, FAKE_MODAL_KEY, FAKE_MODAL_SECRET, 5)
                with following.open(client._request("GET", "/v1/models", "full"), timeout=5) as response:
                    self.assertEqual(response.status, 200)
                origin_requests = list(fake.requests)
        # Control: the stock pattern (Request(headers=...)) leaks to the sink.
        self.assertEqual(sink.requests[0]["Modal-Secret"], FAKE_MODAL_SECRET)
        # SmokeClient's request sent credentials to the origin only.
        self.assertEqual(origin_requests[1]["modal_secret"], FAKE_MODAL_SECRET)
        self.assertEqual(origin_requests[1]["authorization"], f"Bearer {FAKE_API_KEY}")
        self.assertEqual(len(sink.requests), 2)
        self.assertEqual({h: sink.requests[1][h] for h in CREDENTIAL_HEADERS}, dict.fromkeys(CREDENTIAL_HEADERS))


if __name__ == "__main__":
    unittest.main()
