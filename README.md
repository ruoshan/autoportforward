# Auto-portforward (apf)

A handy tool to automatically set up proxies that expose the remote container's listening ports
back to the local machine. Just like `kubectl portforward` or `docker run -p LOCAL:REMOTE`, but
automatically discover and update the ports to be forwarded on the fly.

I often find myself forgot to add `-p` option when testing a image with docker, or missed to
expose some other ports. Now I don't need to worry about that, I just run the following commands:

```
$ docker run -d --name ch clickhouse-server:latest

$ apf ch
LISTENING PORTS: [9005 ==> 9005, 9009 ==> 9009, 8123 ==> 8123, 9000 ==> 9000, 9004 ==> 9004]
```

apf will update the port list on the fly. So if you login to the container and start other
server listening on different ports, it will dynamically update the local listeners.

## Installation

First of all, apf requires a working `docker` client setup, the client can conect to either
local docker daemon or remote.

You can either download the binary from the release artifacts or build it yourself.

```
# MacOS (intel)
curl -L -O https://github.com/ruoshan/autoportforward/releases/download/v0.0.1/apf-mac
chmod +x apf-mac
mv apf-mac /usr/local/bin/apf

# Linux
curl -L -O https://github.com/ruoshan/autoportforward/releases/download/v0.0.1/apf-linux-x64
chmod +x apf-mac
mv apf-mac /usr/local/bin/apf
```

To manually build it, clone the repo and run the `build.sh` script.

## Usage

```
apf {container ID}
```

## Status

- Supporting for kubernetes pod is in progress
