# Worker platforms

## Decision

`codex-fleet` must support Windows 11 as a first-class worker platform. Installing Linux is not a prerequisite for using a Windows laptop as an Ollama worker.

The worker should be distributed as one standalone executable:

- Linux: `codex-fleet-agent`
- Windows: `codex-fleet-agent.exe`

The executable runs in the user's session, without an installer, administrator privileges, system service, Docker, Python, Node.js, or WSL.

## Recommended Windows path

The Windows worker makes an outbound connection to the master host. The master does not need to open an inbound service on the Windows laptop.

```text
codex-fleet-agent.exe
  -> localhost:11434 Ollama
  -> outbound HTTP connection to the master
```

Initial launch:

```powershell
.\codex-fleet-agent.exe agent `
  --master http://MASTER_HOST:8765 `
  --name aorus5-se4
```

The worker reports its identity, Ollama models, active model, queue state, and available capacity. It receives simple prompt jobs and returns structured results.

The Windows worker must not depend on SSH, WinRM, Windows services, registry changes, or shared network drives for the initial workflow.

## Windows remote console options

Windows 11 supports OpenSSH Server (`sshd`) as an optional Windows capability. The normal installation path requires administrator access to add the capability, start the `sshd` service, and allow inbound TCP port 22 through Windows Firewall. Microsoft documents OpenSSH Server as an optional feature on Windows 11 and recommends checking the `sshd` service and firewall rule after installation.

OpenSSH Server is therefore an optional transport for machines where the user can approve one-time administrative setup:

```text
master -> ssh -> Windows user -> Ollama / codex-fleet commands
```

The preferred no-admin path remains an outbound `codex-fleet-agent.exe`. It provides a controlled remote command surface and avoids exposing `sshd`, WinRM, RDP, or an inbound firewall port.

A manually launched, user-mode `sshd` on a high port may be technically possible with a custom configuration, but it adds key, host-key, ACL, and lifecycle complexity. It is not part of the initial deployment path.

WinRM / PowerShell Remoting is also out of scope for the MVP because it requires more Windows policy and listener configuration than the outbound agent.

## WSL2 mode

WSL2 is an optional Linux-worker mode for a Windows laptop. If WSL2 and a Linux distribution are already installed, the Linux `codex-fleet-agent` can run inside WSL2 and use the same worker protocol as a native Linux host.

```text
Windows 11
  -> WSL2 / Ubuntu
       -> codex-fleet-agent
            -> Ollama
            -> outbound connection to the master
```

This is useful when the user already has a Linux-oriented setup or wants to reuse the Linux binary and shell scripts. The validated deployment pattern is a dedicated Linux user inside the WSL2 distribution, an SSH key exchanged with the master, and a stable SSH alias. In that mode the Windows laptop is treated as a Linux worker, just like the Jetson.

Enabling WSL2 normally requires administrator access, Windows optional features, hardware virtualization, and usually a restart. Therefore it cannot satisfy the strict “no admin privileges on a clean laptop” requirement, but it does not require further administrative access after the Linux environment and SSH transport have been configured.

WSL2 does not automatically provide a remote console, but running `sshd` inside WSL2 is an accepted transport for this project because the separate-user and key-exchange workflow is already proven. The implementation must document how the master reaches the WSL2 SSH endpoint: stable host/port mapping, or the Windows networking mode selected by the operator. For the first implementation, treat the worker's Ollama endpoint as an explicit local configuration value and verify it during registration.

## Existing Linux path

Linux hosts may use the already working SSH transport. The first known worker is:

```text
ssh jetson-codex
```

The master can execute discovery and one-shot jobs over this alias. A persistent Linux agent may be added later for push heartbeats and lower-latency scheduling.

## Worker discovery and registration

SSH does not discover unknown hosts. An SSH-based worker must first be added to
the master's registry or SSH configuration. The first MVP therefore uses an
explicit onboarding operation:

```text
operator provides: name, address, SSH user, key, allowed models
master -> SSH discovery -> hostname / ollama list / ollama ps
```

The future user-facing command should make this a single operation, for example:

```bash
codex-fleet worker add \
  --name old-ubuntu-worker \
  --address 192.168.68.80 \
  --user codex \
  --identity ~/.ssh/id_ed25519_codex_fleet
```

After registration, the master stores the worker identity and periodically
refreshes its models, active jobs, and availability. A worker is not considered
available merely because its IP responds to ping or port 22 is open.

For dynamic onboarding, the worker runs a small outbound agent:

```text
worker agent -> master registration endpoint
worker agent -> heartbeat: identity, models, load, capacity
master -> registry and scheduler
```

The agent uses a one-time enrollment token or an approved public key. This avoids
requiring inbound access to every worker and allows the master to see workers
whose DHCP address changes. mDNS/Avahi LAN scanning may be added as an optional
discovery hint, but it is not an authentication or registration mechanism.

NATS can carry registration and heartbeat events later, but it is not required
for the initial SSH-based MVP.

## Linux fallback

Native Linux installation is the preferred fallback when a Windows host cannot run the agent reliably.

A purpose-built Linux Live image is an emergency option only. It adds hardware, boot, persistence, networking, and model-storage complexity and should not be part of the initial deployment path.

## Job scope

The first worker protocol supports prompt/result jobs only:

```text
prompt -> Ollama -> response
```

Repository access, file transfer, Git worktrees, patches, and controlled shell execution are later extensions. They must use explicit workspace allowlists and return patches or artifacts rather than silently sharing a working directory.

## Platform matrix

| Platform | Initial transport | Admin privileges | Initial role |
|---|---|---:|---|
| Linux | SSH alias or outbound agent | No | Ollama prompt worker |
| Windows 11 | Outbound standalone agent | No | Ollama prompt worker |
| Windows 11 + WSL2 | SSH to dedicated WSL2 Linux user, or outbound agent | Initial setup usually yes | Linux-compatible Ollama prompt worker |
| Linux Live | Manual fallback | Boot/setup dependent | Recovery/temporary worker |

## Consequence

The control plane must abstract worker transport. SSH is one Linux transport, while the Windows agent uses the same job protocol over an outbound connection. NATS remains an optional later transport and is not required for the first Windows/Linux MVP.
