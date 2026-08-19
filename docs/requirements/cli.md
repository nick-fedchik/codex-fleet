# Deterministic CLI

## Decision

The first `codex-fleet` interface is a small, deterministic Cobra CLI. It must
be useful from a terminal and from shell scripts before we build a monitoring
TUI, background daemon, or NATS-based control plane.

The CLI must not require a resident master service for the initial SSH workflow.
Every operation is explicit and finishes with a clear exit status.

## Command shape

```text
codex-fleet
  worker add NAME       Register a worker locally
  worker list            List registered workers and last known state
  worker check NAME      Perform a live availability check
  worker check --all     Check every registered worker
  worker inspect NAME    Fetch live identity, models, and runtime details
  worker warmup NAME     Load one model and measure loading time
  worker run NAME        Run one prompt on a selected worker
  worker remove NAME     Remove a worker from the local registry
  config path            Print the active configuration path
  version                Print the CLI version
```

Initial examples:

```bash
codex-fleet worker add WORKER_NAME \
  --ssh-host WORKER_SSH_ALIAS \
  --check

codex-fleet worker list
codex-fleet worker check WORKER_NAME
codex-fleet worker check --all --format json
codex-fleet worker inspect WORKER_NAME
codex-fleet worker warmup WORKER_NAME --model MODEL_NAME
codex-fleet worker run WORKER_NAME \
  --model MODEL_NAME \
  --prompt "Summarize the current worker status." \
  --keep-alive 10m \
  --timeout 10m
```

The exact flags may evolve, but the distinction between registry, live check,
inspection, and execution is intentional and should remain stable.

## Command semantics

### `worker add`

Adds or updates a local worker record. It does not silently run a job. The
operator may request an immediate verification with an explicit `--check` flag.

The record contains transport configuration and an operator label, not secrets:

- stable worker name;
- SSH host or alias;
- SSH user and optional port;
- identity-file reference;
- optional expected model labels.

### `worker list`

Lists the local registry. It must not claim that a worker is currently online
unless a recent live check supports that state. A table is the default output;
`--format json` is required for scripts and future schedulers.

### `worker check`

Performs a bounded live check:

1. connect using the configured transport;
2. verify the remote identity;
3. verify that Ollama responds;
4. return a clear success or failure status.

The command must have a timeout and must never wait indefinitely for a powered-off
or disconnected host. `worker check --all` checks registered workers serially,
updates their last-known state, reports every result, and exits with status `10`
if at least one worker is unavailable.

### `worker inspect`

Fetches a detailed live report without running a model job:

- hostname and user;
- transport latency;
- Ollama version and endpoint status;
- installed models;
- active models and memory usage when available;
- CPU/RAM/GPU information when available;
- current fleet metadata.

### `worker run`

Runs one explicit prompt on one named worker and returns the result. It must
require the worker name and model explicitly in the initial MVP. Streaming,
parallel fan-out, retries, and scheduler-selected workers are later features.
The command keeps the selected model loaded for the configured `--keep-alive`
duration and supports `--format json` with Ollama load and generation metrics.

### `worker warmup`

Explicitly loads one model without running a user prompt. It returns the model
load duration and total request duration, making cold starts measurable. The
model remains loaded for `--keep-alive` unless Ollama is configured otherwise.
Both warmup and run have an explicit `--timeout` so a powered-off host or a
model that cannot load does not wait indefinitely.

## Output and exit status

Human-readable output is the default. Every read or check command should support
stable JSON output for automation:

```bash
codex-fleet worker inspect WORKER_NAME --format json
```

The first implementation uses stable non-zero exit statuses for failure classes:

| Condition | Exit status |
|---|---:|
| Success | 0 |
| Invalid command or arguments | 2 |
| Unknown worker | 3 |
| Worker unavailable or timeout | 10 |
| SSH/transport failure | 11 |
| Ollama unavailable | 12 |
| Remote job failure | 13 |

Error messages go to stderr; command results go to stdout.

## Explicitly out of scope for MVP

- bashtop-like TUI;
- background monitoring service;
- automatic LAN scanning;
- NATS dependency;
- automatic retries and speculative execution;
- repository synchronization or remote shell execution;
- implicit selection of a worker or model.

These features can be added after the deterministic SSH path is exercised on
native Ubuntu and WSL2 workers.
