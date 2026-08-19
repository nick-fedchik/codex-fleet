# codex-fleet

Lightweight distributed worker fleet for Codex, orchestrating local Ollama
models across Linux, Windows, and WSL2 hosts.

## Current status

The first implementation is a deterministic Cobra CLI using SSH as the initial
worker transport. The MVP supports worker registration, live availability checks,
inspection, and explicit prompt execution.

```bash
go run ./cmd/codex-fleet --help

codex-fleet worker add jetson --ssh-host jetson-codex
codex-fleet worker list
codex-fleet worker check jetson
codex-fleet worker inspect jetson
codex-fleet worker run jetson \
  --model MODEL_NAME \
  --prompt "Your prompt"
```

The worker onboarding guides are in [`docs/guides`](docs/guides). Requirements
and implementation state are tracked in
[`docs/requirements/status.md`](docs/requirements/status.md).

Monitoring TUI, dynamic outbound registration, NATS, and repository workspace
jobs are intentionally later milestones.

## License

GNU GPLv3. See [`LICENSE`](LICENSE).
