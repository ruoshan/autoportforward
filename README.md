# Auto-portforward (apf)

A handy tool to automatically set up proxies that expose the remote container's listening ports
back to the local machine. Just like `kubectl portforward` or `docker run -p LOCAL:REMOTE`, but
automatically discover and update the ports to be forwarded on the fly.

I often find myself forgot to add `-p` option when testing a image with docker, or missed to
expose some other ports. Now I don't need to worry about that, I just run the following commands:

```
$ docker run -d --name redis redis

$ apf redis

*  ==> : Forwarding local listening ports to (==>) remote ports
*  <== : Forwarding to local ports from (<==) remote listening ports (use -r option)

Forwarding: [6379 ==> 6379]
```

apf will update the port list on the fly. So if you login to the container and start other
server listening on different ports, it will dynamically update the local listeners.

For kubernetes:

```
$ kubectl run --image redis redis

$ apf -k default/redis
Forwarding: [6379 ==> 6379]
```

## Installation

First of all, apf requires a working `docker` / `kubectl` client setup, the client can conect to either
local docker daemon / k8s cluster or remote.

You can either download the binary from the release artifacts or build it yourself.

```
# MacOS (Intel)
curl -L -O https://github.com/ruoshan/autoportforward/releases/download/v0.0.3/apf-mac
chmod +x apf-mac
mv apf-mac /usr/local/bin/apf

# Linux
curl -L -O https://github.com/ruoshan/autoportforward/releases/download/v0.0.3/apf-linux-x64
chmod +x apf-mac
mv apf-mac /usr/local/bin/apf
```

To manually build it, clone the repo and run the `build.sh` script.

## Usage

### Expose all the listening ports in the container back to the local machine

```
# Docker
apf {container ID / name}

# Kubernetes
apf -k {namespace}/{pod name}
```

### Also expose local ports (8080,9090) to the container

```
# Docker
apf -r 8080,9090  {container ID / name}

# Kubernetes
apf -r 8080,9090 -k {namespace}/{pod name}
```
