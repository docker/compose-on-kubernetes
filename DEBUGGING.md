# How to debug Compose on Kubernetes

## Pre-requisites

- make
- Docker for Desktop (Mac or Windows) Edge with Experimental features enabled (version with support for building with Buildkit)
- Debugger capable of remote debugging with delve api-version 2 (Goland run-configs are pre-configured)

## Quick start

### Debug install

- `make -f debug.Makefile install-debug-images`: this builds the debug-enabled images for the Stack API server and controller, and deploys them replacing any installed instances of Compose on Kubernetes on the local cluster.

### Live debugging

- `make -f debug.Makefile install-live-debug-images`: this builds the debug-enabled images for the Stack server API and controller, and deploys them replacing any installed instances of Compose on Kubernetes on the local cluster (restarting Docker for Desktop will restore the original API and controller). This also exposes the Delve endpoints of the API server and controller on a known port on localhost (40000 for the controller, 40001 for the API). These container images wait for a delve client to be attached before actually running.
- With Goland, select the `Debug all` run config, setup break points and start the debugger to actually start the api and controller processes.
- Alternatively you can setup any delve compatible client to attach to ports 40000 and 40001 on localhost
