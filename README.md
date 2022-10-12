# kmux

[![Build](https://github.com/ViBiOh/kmux/workflows/Build/badge.svg)](https://github.com/ViBiOh/kmux/actions)
[![codecov](https://codecov.io/gh/ViBiOh/kmux/branch/main/graph/badge.svg)](https://codecov.io/gh/ViBiOh/kmux)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=ViBiOh_kube&metric=alert_status)](https://sonarcloud.io/dashboard?id=ViBiOh_kube)

`kmux` is a tool for executing common Kubernetes actions on one or many clusters at the same time.

For example when you have multiple Kubernetes clusters you want to tail the logs of the same deployment simultaneously, or check the image deployed on each cluster.

## Getting started

### Release

Download the latest binary for your os and architecture from the [GitHub Releases page](https://github.com/ViBiOh/kmux/releases)

```bash
curl \
  --disable \
  --silent \
  --show-error \
  --location \
  --max-time 300 \
  --output "/usr/local/bin/kmux"
  https://github.com/ViBiOh/kmux/releases/download/v0.1.0/kmux_$(uname -s | tr "[:upper:]" "[:lower:]")_amd64
chmod +x "/usr/local/bin/kmux"
```

### Golang

```bash
go install "github.com/ViBiOh/kmux@latest"
```

### Shell completions

Shell completions are available by running the following command (example is for `bash`, but it's available for `zsh`, `fish` and `powershell`).

```bash
source <(kmux completion bash)
```

You can also put in a dedicated file and source it from your `*sh.rc`

## Features

Because the goal of this tool is to be used on multiple clusters at once, we rely on high-level object name (object that templatize pods, e.g. deployments, daemonset, etc.).

For running on multiple clusters at once, set the `--context` flag multiple times.

```bash
kmux --context central1 --context europe1 --context asia1 image
```

```
Global Flags:
  -A, --all-namespaces      Find resources in all namespaces
      --context strings     Kubernetes context, multiple for mutiplexing commands
      --kubeconfig string   Kubernetes configuration file (default "${HOME}/.kube/config")
  -n, --namespace string    Override kubernetes namespace in context
```

### `log`

`log` command open a pod's watcher on a resource (Deployment, Service, CronJob, etc) by using label or fiels selector and stream every container of every pod it finds. New pods matching the selector are automatically streamed.

Each log line has a prefix of the pod's name and the container name, and also the context's name if there are multiple contexts. These kind of metadatas are written to the `stderr`, this way, if you have logs in JSON, you can pipe `kmux` output into `jq` for example for extracting wanted data from logs (instead of using `grep`).

The `--containers` can be set multiple times to restrict output to the given containers' name.

```bash
Get logs of a given resource

Usage:
  kmux log TYPE NAME [flags]

Aliases:
  log, logs

Flags:
  -c, --containers strings        Filter container's name, default to all containers, supports regexp
  -d, --dry-run                   Dry-run, print only pods
  -g, --grep string               Regexp to filter log
  -h, --help                      help for log
  -l, --selector stringToString   Labels to filter pods (default [])
  -s, --since duration            Display logs since given duration (default 1h0m0s)
```

### `port-forward`

Like `log`, `port-forward` command open a pod's watcher on a resource and port-forward to every container matching port and being ready. New pods matching the selector are automatically streamed.

A local tcp load-balancer is started on given `local port` that will forward to underlying pods by using round-robin algorithm.

```bash
Port forward to pods of a resource

Usage:
  kmux port-forward TYPE NAME [local_port:]remote_port [flags]

Aliases:
  port-forward, forward

Flags:
  -d, --dry-run   Dry-run, print only pods
  -h, --help      help for port-forward
```

### `watch`

`watch` for all pods in a given namespace (or all namespaces). Status phase is done in a nearly same way that the official `kubectl` (computing the status of a Pod is not that easy).

Output is colored according to the current status of the pod, for better clarity.

```bash
Get all pods in the namespace

Usage:
  kmux watch [flags]

Flags:
  -h, --help                      help for watch
  -o, --output string             Output format. One of: (wide)
  -l, --selector stringToString   Labels to filter pods (default [])
```

### `restart`

`restart` perform the equivalent of a rollout restart on given resource (add an annotation of the pod spec). For `job`, it's the equivalent of a replacement (delete then create).

```bash
Restart pod of the given resources

Usage:
  kmux restart TYPE NAME [flags]

Flags:
  -h, --help   help for restart
```

### `image`

`image` prints the image name of all containers found in given resource. The idea is to check that every cluster runs the same version.

```bash
Get all image names of containers for a given resource

Usage:
  kmux image TYPE NAME [flags]

Flags:
  -h, --help   help for image
```
