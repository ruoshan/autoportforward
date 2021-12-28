# Auto-portforward (apf)

A handy tool to automatically set up proxies that expose the remote container's listening ports
back to the local machine. Just like `kubectl portforward` or `docker run -p LOCAL:REMOTE`, but
automatically discover and update the ports to be forwarded on the fly.

# Installation

You can either download the binary from the release artifacts or build it yourself.

To manually build it, clone the repo and run the `build.sh` script.
