"""Sela LLM server on Modal: Meta Muse Glimmer 30B behind vLLM's OpenAI API.

What this file defines (Modal app "sela-llm"):

* ``serve``: an ``@app.server`` that runs ``vllm serve`` on one H100 and exposes
  the OpenAI-compatible API (``/v1/chat/completions``, ``/v1/models``,
  ``/health``) behind Modal proxy auth (Modal-Key / Modal-Secret headers) and
  a vLLM API key (``Authorization: Bearer``). This is what internal/llm talks to.
* ``download_weights``: a CPU-only function that downloads the pinned model
  revision into a Modal Volume and verifies every file (size plus SHA-256 or
  git blob hash). The GPU server never downloads anything; it only reads the
  verified snapshot, offline.

Sela uses the model for bounded drafting/extraction tasks only. It never
diagnoses, triages, prescribes or acts as a therapist, and every output is
re-validated by the Go application. Nothing here has been deployed from this
repository yet; see modal/README.md (runbook) and modal/MODEL_NOTES.md
(verified model and serving facts, with sources and dates).

Deploy (owner action, costs money; read modal/README.md first):

    modal run    -e dev modal/sela_llm.py::download_weights   # once per environment
    modal deploy -e dev --strategy recreate modal/sela_llm.py

Sources checked on 2026-10-26 (details and quotes in modal/MODEL_NOTES.md):

* @app.server signature: https://modal.com/docs/sdk/py/latest/App (and
  inspect.signature of modal==1.6.2), semantics:
  https://modal.com/docs/guide/servers
* Proxy auth: https://modal.com/docs/guide/webhook-proxy-auth
* Existing images: https://modal.com/docs/guide/existing-images
* GPU (H100 may be upgraded to H200 at the same price):
  https://modal.com/docs/guide/gpu
* Volumes and read-only mounts: https://modal.com/docs/guide/volumes,
  https://modal.com/docs/sdk/py/latest/Volume
* Modal's vLLM pattern: https://modal.com/docs/examples/vllm_inference
* Meta deploy guide: https://dev.meta.ai/docs/muse-glimmer/deploy
* vLLM recipe (declared source of truth by Meta):
  https://recipes.vllm.ai/meta-models/Muse-Glimmer-30B
* vLLM v0.32.0 source (flags, env vars, auth middleware):
  https://github.com/vllm-project/vllm/tree/v0.32.0
"""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import time
import urllib.error
import urllib.request

import modal

# ---------------------------------------------------------------------------
# Pinned identity. The Go app (internal/llm, internal/config) is configured
# with the same three values and refuses to label output "live_meta" unless
# the server reports SERVED_MODEL_NAME.
# ---------------------------------------------------------------------------

APP_NAME = "sela-llm"
# Modal name of the server inside the app. Used by modal.Server.from_name()
# and, per Modal's URL scheme, in the server's URL label.
SERVER_NAME = "serve"

MODEL_ID = "meta-models/Muse-Glimmer-30B"
# Full commit SHA from the Hugging Face API (observed 2026-10-26; the repo
# last changed 2026-10-22, so pinning matters). Apache-2.0, not gated.
MODEL_REVISION = "cec94fcfd6512f2cfe708138c32533b346ea4f69"
# Encodes the short revision so a model swap is visible in every response's
# "model" field and in /v1/models. Must equal SELA_LLM_SERVED_MODEL in Go.
SERVED_MODEL_NAME = "sela-glimmer-cec94fc"

if not SERVED_MODEL_NAME.endswith(MODEL_REVISION[:7]):
    raise RuntimeError("SERVED_MODEL_NAME must encode the short model revision")

# vLLM release pinned by version. 0.32.0 is the latest on PyPI (2026-10-18);
# Muse Glimmer support landed in 0.28.0 (vLLM recipe). We use the official
# vllm/vllm-openai image at the matching tag rather than pip-installing vLLM:
# Meta's deploy guide calls Docker "the supported path", the vLLM recipe
# specifies an image, and the image ships the CUDA 13.0 runtime pieces vLLM's
# JIT kernels need. Modal's host driver exposes CUDA 13.3, which is newer
# than the image's CUDA 13.0, so it is compatible (modal.com/docs/guide/cuda).
# The tag's digest observed on Docker Hub is recorded in MODEL_NOTES.md.
VLLM_VERSION = "0.32.0"
VLLM_IMAGE = f"vllm/vllm-openai:v{VLLM_VERSION}"

# ---------------------------------------------------------------------------
# Capacity and cost bounds.
# ---------------------------------------------------------------------------

# One 80 GB H100. Meta budgets ~72 GB VRAM for the BF16 checkpoint (~59.6 GB
# of weights). Modal may place an "H100" request on an H200 at the same
# price; that only adds KV-cache headroom.
GPU = "H100"

# Sela prompts are short (<4k tokens) and outputs are bounded, so the context
# is capped far below the model's 131072 to keep KV-cache memory small.
# Upper bound for KV cache at full load (inferred from config.json, ignores
# sliding-window savings): 52 layers x 2 KV heads x 128 dims x 2 (K,V) x 2
# bytes = 52 KiB per token; 8 seqs x 16384 tokens x 52 KiB ~= 6.5 GiB.
MAX_MODEL_LEN = 16384
# At most 8 sequences in flight; Modal's max_concurrency matches this so the
# proxy answers 429 instead of letting requests pile up inside vLLM.
MAX_NUM_SEQS = 8
# 0.90 x ~80 GB leaves ~16 GB beyond the weights for KV cache, activations and
# CUDA graphs; 0.90 is also the value in Meta's "tighter memory" command.
# Confirm the "KV cache" and "Maximum concurrency" lines in the startup log.
GPU_MEMORY_UTILIZATION = 0.90

# Meta: reasoning cannot be turned off; "low" spends the fewest thinking
# tokens. This is only the server default (vLLM --default-chat-template-kwargs);
# requests override it with chat_template_kwargs.reasoning_strength.
DEFAULT_REASONING_STRENGTH = "low"

VLLM_PORT = 8000
MINUTES = 60
# Cold start = image load + ~60 GB weights from the Volume + torch.compile
# (cached on a Volume after the first boot) + CUDA graph capture. Expected to
# take minutes; 20 minutes is the hard ceiling before Modal fails the
# container. start() fails much sooner if vLLM exits.
STARTUP_TIMEOUT_S = 20 * MINUTES
# Scale to zero after 5 idle minutes (ADR-0001). Idle minutes are billed.
SCALEDOWN_WINDOW_S = 5 * MINUTES
# Time Modal gives in-flight requests when it drains a container (servers
# guide). Scale-down after idleness has none in flight, so this costs GPU
# time only when a drain interrupts active streams.
EXIT_GRACE_PERIOD_S = 2 * MINUTES
# Proxy region. Supported: us-east (default), us-west, ca-central, eu-west,
# ap-south, ap-southeast-2 (modal.com/docs/guide/servers). Pick the one
# closest to where the Go app runs; this does not change compute pricing.
ROUTING_REGION = "us-east"
# (request, limit). Billing uses the higher of request and actual usage
# (modal.com/docs/guide/resources), so the limits also bound the CPU and
# memory part of the hourly cost. The CPU limit is soft (throttling); the
# memory limit is hard (OOM kill), set far above vLLM's expected host RAM use.
SERVER_CPU = (4.0, 8.0)
SERVER_MEMORY_MIB = (16384, 131072)
DOWNLOAD_CPU = (4.0, 8.0)
DOWNLOAD_MEMORY_MIB = (8192, 32768)

# ---------------------------------------------------------------------------
# Secrets, Volumes and paths. Secrets and Volumes are looked up by name in
# the Modal environment chosen at deploy time (`modal deploy -e <env>`), so
# dev and staging stay isolated and no workspace is hard-coded.
# ---------------------------------------------------------------------------

# Modal Secret holding the vLLM API key (the value the Go app sends as
# "Authorization: Bearer"). Create it per environment; see README.md.
API_KEY_SECRET_NAME = "sela-llm-api"
API_KEY_ENV = "VLLM_API_KEY"
# Refuse to start with a weak or empty key (fail closed).
MIN_API_KEY_LENGTH = 32
# Optional: the model is ungated, so no Hugging Face token is needed. Set this
# local environment variable to the name of a Modal Secret with key HF_TOKEN
# when running download_weights if you want authenticated downloads.
HF_SECRET_NAME_ENV = "SELA_HF_SECRET_NAME"

HF_CACHE_VOLUME_NAME = "sela-hf-cache"
VLLM_CACHE_VOLUME_NAME = "sela-vllm-cache"
HF_CACHE_DIR = "/root/.cache/huggingface"
VLLM_CACHE_DIR = "/root/.cache/vllm"
# The server mounts the weights read-only, so it gets a separate, writable
# (and empty) HF_HOME for anything a library might want to write.
SCRATCH_HF_HOME = "/tmp/sela-hf-home"

# Every file of the pinned revision with its size and digest, from the Hugging
# Face tree API at MODEL_REVISION (observed 2026-10-26). "sha256" is the LFS
# object id; "git-sha1" is the git blob id (sha1 of "blob <size>\0" + bytes).
EXPECTED_FILES: dict[str, tuple[int, str, str]] = {
    ".eval_results/mmi.yaml": (1285, "git-sha1", "09051e27cbbc2431dec7753abbf927ba0d80fd1c"),
    ".gitattributes": (1570, "git-sha1", "52373fe24473b1aa44333d318f578ae6bf04b49b"),
    "LICENSE": (11358, "git-sha1", "d645695673349e3947e8e5ae42332d0ac3164cd7"),
    "README.md": (17266, "git-sha1", "2e8eff65762ef99e79d60ef4cbdb338a4b292d6f"),
    "USAGE_POLICY.md": (5230, "git-sha1", "1a9ed6cffc54585daf9cc904c07cb6e436a9444b"),
    "chat_template.jinja": (9992, "git-sha1", "7507f3c9f38809152732c045df3977848f3916a6"),
    "config.json": (5109, "git-sha1", "190826dc834c13c86b8dc68e775a888a852d7c34"),
    "generation_config.json": (202, "git-sha1", "b69a50a4239f42707d54a479e0143b8ee56c8bcc"),
    "model-00001-of-00002.safetensors": (
        49950112952,
        "sha256",
        "8eef61530e1283642c77ce2e6721feb5c6f348fa055c00e90f2844a136372694",
    ),
    "model-00002-of-00002.safetensors": (
        9603322320,
        "sha256",
        "b58cc2144ba1ba1af4420f67f4ca3ced7f09298510b80464cc75018a0be14381",
    ),
    "model.safetensors.index.json": (132674, "git-sha1", "00b257ef0c8bce1b001a6266f6b1f66f4fe16a6b"),
    "processor_config.json": (1084, "git-sha1", "ec9a07be13ea1dadf7395131871753b13f406c26"),
    "tokenizer.json": (
        28129897,
        "sha256",
        "c9dbee66967b58f31a7c27f723c3760da3526ccd0427578e8905b0abb0031c4d",
    ),
    "tokenizer_config.json": (79936, "git-sha1", "fe7f0bb90fb0d0288e3a505974ae0e4baf1d3e61"),
}

# Environment for the GPU server process.
SERVER_ENV = {
    "HF_HOME": SCRATCH_HF_HOME,
    # Never contact the Hub at runtime: the verified snapshot is all we serve.
    "HF_HUB_OFFLINE": "1",
    "HF_HUB_DISABLE_TELEMETRY": "1",
    "DO_NOT_TRACK": "1",
    # vLLM usage-stats reporting off (vllm/envs.py at v0.32.0).
    "VLLM_NO_USAGE_STATS": "1",
    "VLLM_DO_NOT_TRACK": "1",
    # torch.compile and other vLLM caches go to the persistent Volume.
    "VLLM_CACHE_ROOT": VLLM_CACHE_DIR,
    # INFO, never DEBUG: at DEBUG, vLLM's request logger (if ever enabled)
    # would print prompt inputs.
    "VLLM_LOGGING_LEVEL": "INFO",
}

# Environment for the CPU download job.
DOWNLOAD_ENV = {
    "HF_HOME": HF_CACHE_DIR,
    "HF_HUB_DISABLE_TELEMETRY": "1",
    "HF_HUB_DISABLE_PROGRESS_BARS": "1",
    # Saturate the container's network for the ~60 GB transfer (hf-xet).
    "HF_XET_HIGH_PERFORMANCE": "1",
}


def snapshot_dir(cache_dir: str = HF_CACHE_DIR) -> str:
    """Hugging Face cache path of the pinned snapshot (standard cache layout)."""
    repo_folder = "models--" + MODEL_ID.replace("/", "--")
    return f"{cache_dir}/hub/{repo_folder}/snapshots/{MODEL_REVISION}"


def verified_marker_path(cache_dir: str = HF_CACHE_DIR) -> str:
    """Marker written by download_weights after a full digest verification."""
    return f"{cache_dir}/sela-verified/{MODEL_REVISION}.json"


def git_blob_sha1(path: str) -> str:
    """Git blob id of a file, as the Hugging Face API reports for non-LFS files."""
    digest = hashlib.sha1(usedforsecurity=False)
    digest.update(b"blob %d\0" % os.path.getsize(path))
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(8 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(8 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_snapshot(directory: str, deep: bool, expected: dict | None = None) -> list[str]:
    """Return a list of problems (empty when the snapshot matches the pin).

    Shallow mode checks presence and sizes only (cheap; used on every GPU cold
    start). Deep mode also checks digests (reads ~60 GB; used by the CPU
    download job, never on the GPU).
    """
    problems = []
    for rel_path, (size, algorithm, digest) in sorted((expected or EXPECTED_FILES).items()):
        path = os.path.join(directory, rel_path)
        if not os.path.isfile(path):
            problems.append(f"missing: {rel_path}")
            continue
        actual_size = os.path.getsize(path)
        if actual_size != size:
            problems.append(f"size mismatch: {rel_path} ({actual_size} != {size})")
            continue
        if deep:
            actual = sha256_file(path) if algorithm == "sha256" else git_blob_sha1(path)
            if actual != digest:
                problems.append(f"{algorithm} mismatch: {rel_path}")
    return problems


def build_vllm_command(model_path: str) -> list[str]:
    """The exact vLLM command line. Contains no secrets.

    Flag names verified against vLLM v0.32.0 source (vllm/engine/arg_utils.py,
    vllm/entrypoints/launchers/cli_args.py) and Meta's deploy guide.
    """
    return [
        "vllm",
        "serve",
        # Local, verified snapshot path: no Hub lookups; /v1/models reports it
        # as "root", which contains the full revision.
        model_path,
        "--served-model-name",
        SERVED_MODEL_NAME,
        # Modal requires the server to bind 0.0.0.0 on the configured port.
        "--host",
        "0.0.0.0",
        "--port",
        str(VLLM_PORT),
        "--tensor-parallel-size",
        "1",
        # Meta/vLLM recipe flags. Parser names use underscores.
        # generation-config auto: checkpoint sampling defaults (temperature
        # 1.0, top_p 0.95, top_k 64) and stop tokens [200001, 200008].
        "--generation-config",
        "auto",
        # Routes the to=self thinking channel to delta.reasoning instead of
        # content. Sela discards reasoning.
        "--reasoning-parser",
        "muse_glimmer",
        # The recipe runs the tool-call parser together with the reasoning
        # parser (both key off the same channel framing). Sela sends no tools.
        "--enable-auto-tool-choice",
        "--tool-call-parser",
        "muse_glimmer",
        # Server-wide default reasoning strength; requests may override it.
        "--default-chat-template-kwargs",
        json.dumps({"reasoning_strength": DEFAULT_REASONING_STRENGTH}),
        "--max-model-len",
        str(MAX_MODEL_LEN),
        "--max-num-seqs",
        str(MAX_NUM_SEQS),
        "--gpu-memory-utilization",
        str(GPU_MEMORY_UTILIZATION),
        # Text only: vLLM skips the ~1.8B-parameter vision tower when every
        # modality limit is zero (frees VRAM) and image inputs are rejected.
        "--language-model-only",
        # Request logging is off by default in 0.32 (--enable-log-requests is a
        # BooleanOptionalAction defaulting to False); state it explicitly so a
        # future default change cannot start logging prompts. (The old
        # --disable-log-requests flag no longer exists in 0.32.)
        "--no-enable-log-requests",
        # No per-request access log lines in Modal logs.
        "--disable-uvicorn-access-log",
        # No /docs or /openapi.json.
        "--disable-fastapi-docs",
        # Deliberately absent: --api-key. vLLM 0.32 reads VLLM_API_KEY from
        # the environment when --api-key is not given
        # (vllm/entrypoints/serve/middleware/register.py), which keeps the key
        # out of argv and out of this logged command line.
    ]


def wait_until_healthy(
    process,
    timeout_s: float,
    health_url: str,
    poll_interval_s: float = 2.0,
    clock=time.monotonic,
    sleep=time.sleep,
) -> float:
    """Block until vLLM answers 200 on /health. Returns the seconds waited.

    Raises RuntimeError as soon as the vLLM process exits (bad flag, OOM,
    missing file), so a broken configuration costs seconds of GPU time
    instead of the full startup_timeout. Raises TimeoutError after timeout_s.
    """
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    started = clock()
    while True:
        return_code = process.poll()
        if return_code is not None:
            raise RuntimeError(f"vLLM exited with code {return_code} before becoming healthy")
        try:
            with opener.open(health_url, timeout=5) as response:
                if response.status == 200:
                    return clock() - started
        except (urllib.error.URLError, OSError):
            pass
        if clock() - started > timeout_s:
            raise TimeoutError(f"vLLM not healthy after {timeout_s:.0f}s")
        sleep(poll_interval_s)


def terminate_process(process, grace_s: float = 25.0) -> None:
    """SIGTERM, then SIGKILL after grace_s (Modal allows ~30s for exit hooks)."""
    if process is None or process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=grace_s)
    except subprocess.TimeoutExpired:
        process.kill()


# ---------------------------------------------------------------------------
# Modal objects. Everything below is lazy until `modal deploy` / `modal run`;
# importing this file does not contact Modal.
# ---------------------------------------------------------------------------

app = modal.App(APP_NAME)

vllm_image = (
    modal.Image.from_registry(
        VLLM_IMAGE,
        # Modal's image builder expects `python` and `pip` on PATH for images
        # without add_python (modal 1.6.2 Image.from_registry docstring). The
        # vLLM image installs python3.12 (+ pip via get-pip) but no unversioned
        # `python`, so link it and fail the build early if either is missing.
        setup_dockerfile_commands=[
            'RUN ln -sf "$(command -v python3)" /usr/local/bin/python'
            " && python --version && python -m pip --version",
        ],
    )
    # The image's ENTRYPOINT is ["vllm", "serve"], which would swallow Modal's
    # own container command; Modal requires an entrypoint that execs "$@".
    .entrypoint([])
)

hf_cache_volume = modal.Volume.from_name(HF_CACHE_VOLUME_NAME, create_if_missing=True)
vllm_cache_volume = modal.Volume.from_name(VLLM_CACHE_VOLUME_NAME, create_if_missing=True)

# Referenced by name only; the value lives in Modal. Deploy fails if the
# Secret or the key is missing in the target environment.
api_key_secret = modal.Secret.from_name(API_KEY_SECRET_NAME, required_keys=[API_KEY_ENV])


def _optional_hf_secret():
    """One Secret for the download job: a named HF token Secret if configured.

    Follows Modal's documented local/remote pattern (modal.com/docs/guide/
    global-variables and Secret.from_local_environ in modal 1.6.2): decorator
    arguments are evaluated both locally and in the container, so the
    container side gets an empty placeholder and the dependency count stays
    the same. The empty placeholder carries no values.
    """
    name = os.environ.get(HF_SECRET_NAME_ENV, "").strip() if modal.is_local() else ""
    if name:
        return modal.Secret.from_name(name, required_keys=["HF_TOKEN"])
    return modal.Secret.from_dict({})


@app.function(
    image=vllm_image,
    volumes={HF_CACHE_DIR: hf_cache_volume},
    secrets=[_optional_hf_secret()],
    env=DOWNLOAD_ENV,
    cpu=DOWNLOAD_CPU,
    memory=DOWNLOAD_MEMORY_MIB,
    timeout=2 * 60 * MINUTES,
    max_containers=1,
)
def download_weights(deep_verify: bool = False) -> dict:
    """Download the pinned revision into the HF cache Volume and verify it.

    Idempotent: if a verified marker exists and sizes still match, it returns
    without downloading (pass --deep-verify to re-hash everything). Runs on
    CPU only. `modal run -e <env> modal/sela_llm.py::download_weights`
    """
    from huggingface_hub import snapshot_download

    target = snapshot_dir()
    marker = verified_marker_path()
    if os.path.isfile(marker) and not deep_verify:
        problems = verify_snapshot(target, deep=False)
        if not problems:
            print(f"Already verified: {MODEL_ID}@{MODEL_REVISION}")
            return {"model_id": MODEL_ID, "revision": MODEL_REVISION, "downloaded": False}
        print("Verified marker present but files changed; re-downloading:", problems)

    started = time.monotonic()
    path = snapshot_download(MODEL_ID, revision=MODEL_REVISION, cache_dir=f"{HF_CACHE_DIR}/hub")
    if os.path.realpath(path) != os.path.realpath(target):
        raise RuntimeError(f"unexpected snapshot path {path!r}; expected {target!r}")
    print(f"Downloaded in {time.monotonic() - started:.0f}s; verifying digests (reads ~60 GB)")

    problems = verify_snapshot(target, deep=True)
    if problems:
        raise RuntimeError("snapshot verification failed: " + "; ".join(problems))

    os.makedirs(os.path.dirname(marker), exist_ok=True)
    record = {
        "model_id": MODEL_ID,
        "revision": MODEL_REVISION,
        "files": len(EXPECTED_FILES),
        "total_bytes": sum(size for size, _, _ in EXPECTED_FILES.values()),
        "verified_at_unix": int(time.time()),
    }
    tmp_marker = marker + ".tmp"
    with open(tmp_marker, "w", encoding="utf-8") as handle:
        json.dump(record, handle)
    os.replace(tmp_marker, marker)
    hf_cache_volume.commit()
    print(f"Verified {MODEL_ID}@{MODEL_REVISION} ({record['total_bytes']} bytes)")
    return {"model_id": MODEL_ID, "revision": MODEL_REVISION, "downloaded": True}


@app.server(
    name=SERVER_NAME,
    image=vllm_image,
    gpu=GPU,
    cpu=SERVER_CPU,
    memory=SERVER_MEMORY_MIB,
    # Weights are read-only for the server; the compile cache is writable.
    volumes={
        HF_CACHE_DIR: hf_cache_volume.with_mount_options(read_only=True),
        VLLM_CACHE_DIR: vllm_cache_volume,
    },
    secrets=[api_key_secret],
    env=SERVER_ENV,
    port=VLLM_PORT,
    # Scale to zero: no GPU is billed while nobody calls the API.
    min_containers=0,
    # Hard cost ceiling: never more than one GPU container. Trade-off (Modal
    # servers guide): no overlapping replacement during a rolling redeploy,
    # so deploy with --strategy recreate and expect a short outage.
    max_containers=1,
    # Autoscaling target and hard per-container limit both equal vLLM's
    # --max-num-seqs; the 9th concurrent request gets a 429 from Modal.
    target_concurrency=MAX_NUM_SEQS,
    max_concurrency=MAX_NUM_SEQS,
    scaledown_window=SCALEDOWN_WINDOW_S,
    startup_timeout=STARTUP_TIMEOUT_S,
    exit_grace_period=EXIT_GRACE_PERIOD_S,
    routing_region=ROUTING_REGION,
    # Proxy auth stays ON (the default; stated explicitly). Requests without
    # a valid Modal proxy token are rejected with 401 by Modal before they
    # reach (or start) a GPU container.
    unauthenticated=False,
)
class SelaLLMServer:
    @modal.enter()
    def start(self) -> None:
        # Fail closed: never serve without the second auth layer.
        if len(os.environ.get(API_KEY_ENV, "")) < MIN_API_KEY_LENGTH:
            raise RuntimeError(
                f"{API_KEY_ENV} (Modal Secret {API_KEY_SECRET_NAME!r}) is missing or shorter "
                f"than {MIN_API_KEY_LENGTH} characters; refusing to start"
            )
        model_path = snapshot_dir()
        problems = verify_snapshot(model_path, deep=False)
        if problems or not os.path.isfile(verified_marker_path()):
            raise RuntimeError(
                "pinned weights are missing or unverified in Volume "
                f"{HF_CACHE_VOLUME_NAME!r}; run download_weights in this environment first "
                f"({len(problems)} problem(s): {problems[:3]})"
            )

        command = build_vllm_command(model_path)
        print("Starting vLLM:", " ".join(command), flush=True)
        self.process = subprocess.Popen(command)
        try:
            waited = wait_until_healthy(
                self.process,
                timeout_s=STARTUP_TIMEOUT_S - MINUTES,
                health_url=f"http://127.0.0.1:{VLLM_PORT}/health",
            )
        except Exception:
            terminate_process(self.process)
            raise
        print(f"vLLM healthy after {waited:.0f}s serving {SERVED_MODEL_NAME}", flush=True)

    @modal.exit()
    def stop(self) -> None:
        terminate_process(getattr(self, "process", None))
