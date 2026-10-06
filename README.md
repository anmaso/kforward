# kforward

A small CLI for managing Kubernetes port forwards. It starts `kubectl port-forward` processes in the background, tracks them, and lets you list, recreate and stop them without keeping a terminal open.

## Requirements

- Go 1.22+
- `kubectl` on your `PATH`
- A working kubeconfig (used to discover services and deployments)
- `lsof` and `ps` (used to detect what is occupying a local port; macOS/Linux)

## Install

```bash
go build -o kforward .
```

Move the resulting `kforward` binary somewhere on your `PATH`.

## Usage

Running `kforward` with no arguments prints help and opens an interactive menu (status / add / remove).

### `kforward add [name] [local:remote]`

Create a new port forward to a service or deployment.

```bash
# Find resources matching "api"; pick one and a port interactively if needed
kforward add api

# Forward localhost:8080 -> remote port 80
kforward add api 8080:80

# Browse every service and deployment in the cluster
kforward add --interactive
```

The name is matched against services and deployments, first in the current namespace and then across all namespaces. If several resources match you are asked to choose. If the local port is already in use, kforward tells you what is using it and offers to kill that process. Ports it manages itself are never killed; remove them first.

### `kforward status`

List all tracked forwards with their state (`up`/`down`), namespace, name, ports and PID. If any are down, you are offered the choice to recreate one or all of them.

```bash
kforward status
kforward status --no-recreate   # list only, no recreate prompt
```

### `kforward remove`

Pick an active forward and stop it.

## How it works

Each forward is a detached `kubectl port-forward` process (own session, so it survives the terminal closing). kforward records it as a PID file named `<namespace>__<name>__<local>__<remote>` in:

```
~/.config/kforward/port-forwards/
```

`status` checks whether each recorded PID is still alive; recreating a forward re-resolves the target in the cluster and starts a fresh process on the same ports.

## Project layout

```
main.go                 entry point
cmd/                    cobra commands (add, status, remove, interactive menu)
internal/discovery/     cluster lookups for services and deployments
internal/forward/       starting, stopping and inspecting port-forward processes
internal/selector/      interactive prompts (huh)
internal/state/         PID-file bookkeeping
internal/config/        config paths
internal/term/          status badge rendering
```
