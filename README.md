# codex-fleet

Lightweight distributed worker fleet for Codex, orchestrating local Ollama
models across Linux, Windows, and WSL2 hosts.

## Current status

The first implementation is a deterministic Cobra CLI using SSH as the initial
worker transport. The MVP supports worker registration, live availability checks,
inspection, model warmup, and explicit prompt execution.

The current control flow is:

```text
master CLI -> SSH -> Linux/WSL2 worker -> local Ollama
```

The worker does not need to build or run this repository. It needs only an SSH
server, `curl`, Ollama, and the requested models. The onboarding guides are in
[`docs/guides`](docs/guides). Requirements and implementation state are tracked
in [`docs/requirements/status.md`](docs/requirements/status.md).

The target worker onboarding contract is simpler: the worker owner should need
to know only the master's hostname or IP address. The current SSH transport is
a transitional fallback and still requires the master operator to know the
worker's SSH address. Outbound registration with master-only bootstrap is tracked
as CF-009.

Monitoring TUI, dynamic outbound registration, NATS, native Windows agent, and
repository workspace jobs are intentionally later milestones.

## Compatibility

The minimum supported Ubuntu release for the master/build machine and native
Linux SSH workers is **Ubuntu 24.04 LTS**. Newer Ubuntu releases are supported;
Ubuntu 26.04 LTS is the recommended baseline for a fresh installation.

The current module requires Go 1.25 or newer. Ubuntu 26.04 provides Go 1.26
from the official Ubuntu repositories, so the complete master setup is:

```bash
sudo apt update
sudo apt install -y git openssh-client golang-go
```

On Ubuntu 24.04, verify `go version` before building. The default Ubuntu Go
package may be older than 1.25, so install Go 1.25+ using the [official Go
installation instructions](https://go.dev/doc/install) when necessary.

## Dependencies

### Master / build machine

- Git;
- Go 1.25 or newer;
- OpenSSH client (`ssh`; `ssh-copy-id` is optional for already-administered hosts).

The only Go module dependency is Cobra, declared in [`go.mod`](go.mod). Go
downloads it automatically; do not install Cobra, Python, Node.js, Docker, or
NATS separately.

On Ubuntu, install the system tools with:

```bash
sudo apt update
sudo apt install -y git openssh-client
```

Install Go 1.25+ using the [official Go installation instructions](https://go.dev/doc/install),
then verify:

```bash
go version
```

### Worker machine

For the current SSH MVP, the worker needs:

- OpenSSH server;
- `curl`;
- Ollama;
- at least one compatible Ollama model.

The worker does not need Go, Git, Cobra, Codex, NATS, or this source tree. Follow
the [Ubuntu worker guide](docs/guides/ubuntu-ssh-ollama-worker.md) or the
[Windows 11 + WSL2 guide](docs/guides/wsl2-ssh-worker.md).

For Ubuntu, the repository includes a dependency installer for a worker whose
dedicated login already exists. Run it as that user and pass only the master's
hostname or IP address:

```bash
curl -fsSL https://raw.githubusercontent.com/nick-fedchik/codex-fleet/main/scripts/install-ubuntu-worker.sh \
  -o install-ubuntu-worker.sh
chmod 0755 install-ubuntu-worker.sh
./install-ubuntu-worker.sh MASTER_HOST
```

The script checks Ubuntu 24.04+, installs SSH and Ollama, enables their services,
creates a worker key, and writes `~/.config/codex-fleet/worker.env`. It does not
create a user, download an unspecified model, or change the master's SSH keys.
During an interactive run it can add the master's one-line public key to the
current user's `authorized_keys`.
For the current inbound SSH transport, generate the access key on the master and
copy only its `.pub` file to the worker's `authorized_keys`; see the
[Ubuntu worker guide](docs/guides/ubuntu-ssh-ollama-worker.md).

If the master is shared by multiple people or services, run `codex-fleet` under
a dedicated local account without `sudo`. Generate the SSH key as that account;
do not use `sudo ssh-keygen`, because that creates a root-owned key. A separate
master account is optional when the master is a single-user laptop.

## Build from source

Clone the repository and enter its directory:

```bash
git clone https://github.com/nick-fedchik/codex-fleet.git
cd codex-fleet
```

Download Go modules, run tests and static checks, then build:

```bash
go mod download
go test ./...
go vet ./...
go build -trimpath -ldflags "-s -w" -o bin/codex-fleet ./cmd/codex-fleet
```

Run the binary directly:

```bash
./bin/codex-fleet --help
```

Install it for the current user without root:

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 bin/codex-fleet "$HOME/.local/bin/codex-fleet"
```

If `$HOME/.local/bin` is not in `PATH`, add it for the current shell:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Configure SSH and add a worker

The master must be able to connect to the worker using an SSH alias. Example
`~/.ssh/config` entry:

```sshconfig
Host worker-alias
    HostName WORKER_IP_OR_DNS_NAME
    User WORKER_SSH_USER
    IdentityFile ~/.ssh/id_ed25519_codex_fleet
    IdentitiesOnly yes
```

Verify SSH first:

```bash
ssh worker-alias 'hostname; id -un; ollama list'
```

Register and verify the worker:

```bash
codex-fleet worker add WORKER_NAME \
  --ssh-host worker-alias \
  --check
```

The local registry path is printed by:

```bash
codex-fleet config path
```

## Operate the worker pool

```bash
codex-fleet worker list
codex-fleet worker check WORKER_NAME
codex-fleet worker check --all --format json
codex-fleet worker inspect WORKER_NAME --format json
```

Warm up a large model before sending work:

```bash
codex-fleet worker warmup WORKER_NAME \
  --model MODEL_NAME \
  --keep-alive 10m \
  --timeout 10m \
  --format json
```

Run a prompt and receive timing metrics:

```bash
codex-fleet worker run WORKER_NAME \
  --model MODEL_NAME \
  --prompt "Reply with exactly: worker online" \
  --keep-alive 10m \
  --timeout 10m \
  --format json
```

If a worker is powered off, `worker check` returns a non-zero exit status. A
successful SSH check only proves that SSH and the Ollama API respond; a large
model may still need several minutes to load. Use `worker warmup` to measure
that separately.

## License

GNU GPLv3. See [`LICENSE`](LICENSE).
