# kube

[![Build](https://github.com/ViBiOh/kube/workflows/Build/badge.svg)](https://github.com/ViBiOh/kube/actions)
[![codecov](https://codecov.io/gh/ViBiOh/kube/branch/main/graph/badge.svg)](https://codecov.io/gh/ViBiOh/kube)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=ViBiOh_kube&metric=alert_status)](https://sonarcloud.io/dashboard?id=ViBiOh_kube)

`kube` is a tool for executing common Kubernetes actions on one or many clusters at the same time.

For example when you have multiple Kubernetes clusters you want to tail the logs of the same deployment simultaneously, or check the image deployed on each cluster.

## Getting started

### Release

Download the latest binary for your os and architecture from the [GitHub Releases page](https://github.com/ViBiOh/kube/releases)

```bash
curl --disable --silent --show-error --location --max-time 300 --output "/usr/local/bin/kube" -- https://github.com/ViBiOh/kube/releases/download/v0.0.1/kube_$(uname -s | tr "[:upper:]" "[:lower:]")_amd64
chmod +x "/usr/local/bin/kube"
```

### Golang

```bash
go install "github.com/ViBiOh/kube@latest"
```

### {ba,z}sh-completion

Shell completions are available by running the following command (example is for `bash`, but it's available for `zsh`, `fish` and `powershell`).

```bash
source <(./bin/kube completion bash)
```

You can also put in a dedicated file and source it from your `*sh.rc`

## Features

Because the goal of this tool is to be used on multiple clusters at once, we rely on high-level object name (object that templatize pods, e.g. deployments, daemonset, etc.).

For running on multiple clusters at once, set a comma-separated value on the `--context` flags.

```bash
kube --context central1,europe1,asia1 image
```

```
Global Flags:
      --context string      Kubernetes context, comma separated for mutiplexing commands
  -h, --help                help for kube
      --kubeconfig string   Kubernetes configuration file (default "/Users/macbook/.kube/config")
  -n, --namespace string    Override kubernetes namespace in context
```

### `log`

`log` command open a pod's watcher on a resource (Deployment, Service, CronJob, etc) by using label selector and stream every containers of every pod it finds. New pods matching the selector are automatically streamed.

Each log line has a prefix of the pod's name and the container name, and also the context's name if there are multiple contexts. These kind of metadatas are written to the `stderr`, this way, if you have logs in JSON, you can pipe `kube` output into `jq` for example for extracting wanted data from logs (instead of using `grep`).

The `--containers` can be set multiple times to restrict output to the given containers' name.

```
Get logs of a given resource

Usage:
  kube log <resource_type> <resource_name> [flags]

Aliases:
  log, logs

Flags:
  -c, --containers strings   Filter container's name, default to all containers
  -d, --dry-run              Dry-run, print only pods
  -h, --help                 help for log
  -s, --since duration       Display logs since given duration (default 1h0m0s)
```

### `restart`

`restart` performat the equivalent of a rollout restart on given resource (add an annotation of the pod spec). For `job`, it's the equivalent of a replace (delete then create).

```
Restart pod of the given resources

Usage:
  kube restart <resource_type> <resource_name>
```

### `image`

`image` prints the image name of all containers found in given resource. The idea is to check that every cluster runs the same version.

```
Get image name of containers for a given resource

Usage:
  kube image <resource_type> <resource_name>
```
