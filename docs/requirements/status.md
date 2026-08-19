# Requirements status

This register is the short project-level view of implementation progress. Each
requirement has an explicit state and an acceptance condition.

Status values:

- `open` — defined, not started;
- `in-progress` — actively being implemented;
- `done` — acceptance condition verified;
- `blocked` — cannot progress without an external decision or dependency.

| ID | Requirement | Status | Acceptance condition |
|---|---|---|---|
| CF-001 | Deterministic Cobra CLI | done | `codex-fleet --help` and stable subcommands build and run |
| CF-002 | Local worker registry | done | add/list/remove persist worker records without secrets |
| CF-003 | Live worker availability check | done | bounded SSH + Ollama check returns a stable exit status on a registered Linux worker |
| CF-004 | Worker inspection | done | hostname, user, Ollama models and runtime state are readable on a registered worker |
| CF-005 | Single prompt execution | done | explicit model/prompt returns a known response from a registered Ollama worker |
| CF-006 | Native Ubuntu onboarding | done | public SSH/Ollama guide and one-argument Ubuntu worker installer exist |
| CF-007 | Windows 11 native worker | open | standalone agent runs without WSL or admin privileges |
| CF-008 | Windows 11 + WSL2 worker | done | public WSL2 + dedicated-user SSH guide exists |
| CF-009 | Dynamic worker registration | open | worker bootstrap requires only master hostname/IP, then outbound agent registers and sends heartbeats |
| CF-010 | Scheduler and worker pool | open | master selects an available compatible worker |
| CF-011 | TUI monitoring view | open | later bashtop-like read-only operational view |
| CF-012 | NATS transport | open | optional event transport is introduced after the SSH MVP |
| CF-013 | Repository/file workspace jobs | open | explicit allowlisted workspace and artifact/patch protocol |

The current implementation target is CF-001 through CF-005. CF-007 and CF-009
must not delay the first SSH-based end-to-end result.
