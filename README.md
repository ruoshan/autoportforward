# Auto-portforward (apf)

A handy tool to automatically set up proxies that expose the remote container's listening ports
back to the local machine. Just like `kubectl portforward` or `docker run -p LOCAL:REMOTE`, but
automatically discover and update the ports to be forwarded on the fly. `apf` can create listening
ports in the container and forward them back as well.

I often find myself forgetting to add `-p` option when testing a docker image or missing to
expose some other ports. Now I don't need to worry about that, I just run the following commands:

```
$ docker run -d --name redis redis

$ apf redis

*  ==> : Forwarding local listening ports to (==>) remote ports
*  <== : Forwarding to local ports from (<==) remote listening ports (use -r option)

Forwarding: [6379 ==> 6379]
```

`apf` will update the port list on the fly. So if you login to the container and start other
server listening on different ports, it will dynamically update the local listeners.

For Kubernetes:

```
$ kubectl run --image redis redis

$ apf -k default/redis
Forwarding: [6379 ==> 6379]
```

For Podman:

```
$ podman run --name redis docker.io/library/redis:latest

$ apf -p redis
Forwarding: [6379 ==> 6379]
```

## Installation

First of all, `apf` requires a working `docker` / `kubectl` client setup, the client can connect to either
local docker daemon / k8s cluster or remote.

You can either download the binary from the release artifacts or build it yourself.

```
# MacOS (Intel)
curl -L -O https://github.com/ruoshan/autoportforward/releases/latest/download/apf-mac
chmod +x apf-mac
mv apf-mac /usr/local/bin/apf

# MacOS (ARM)
curl -L -O https://github.com/ruoshan/autoportforward/releases/latest/download/apf-mac-arm64
chmod +x apf-mac-arm64
mv apf-mac-arm64 /usr/local/bin/apf

# Linux
curl -L -O https://github.com/ruoshan/autoportforward/releases/latest/download/apf-linux-x64
chmod +x apf-linux-x64
mv apf-linux-x64 /usr/local/bin/apf
```

To manually build it, clone the repo and run the `build.sh` script.

## Usage

### Expose all the listening ports in the container back to the local machine

```
# Docker
apf {container ID / name}

# Kubernetes
apf -k {namespace}/{pod name}

# Podman
apf -p {podman container ID / name}
```

### Also expose local ports (8080,9090) to the container

```
# Docker
apf -r 8080,9090  {container ID / name}

# Kubernetes
apf -r 8080,9090 -k {namespace}/{pod name}

# Podman
apf -r 8080,9090 -p {podman container ID / name}
```


## Limitations

- Currently, `apf` only supports linux container(x64 arch)
- For Kubernetes, the container must have `tar` installed

## Tips

1. apf does not come with shell completion, but here is what I do to make it more handy:

```
# `brew install fzf`
alias ap='docker ps | grep -v "^CONTAINER ID" | fzf | awk "{print \$1}" | xargs -n 1 apf'
```
