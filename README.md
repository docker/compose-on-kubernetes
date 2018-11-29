# Compose on Kubernetes

[![CircleCI](https://circleci.com/gh/docker/compose-on-kubernetes/tree/master.svg?style=svg)](https://circleci.com/gh/docker/compose-on-kubernetes/tree/master)

Compose on Kubernetes tools add support of Docker Compose files in Kubernetes.

## Architecture

This project brings `compose.docker.com/v1beta1` and `compose.docker.com/v1beta2` APIs to your cluster.

See the [architecture document](docs/architecture.md) for more information.

## Running for local development

First ensure that you have a running Kubernetes cluster set up.

Perform a local build of the binaries.

```bash
$ make binaries
```

Setup the cluster for local development.

```bash
$ make setup-api-server-dev
```

Run the API server.

```bash
$ make run-api-server-dev
```

Run the `compose-controller`.

```bash
$ ./bin/compose-controller
```

You should now be able to deploy stacks to your cluster using `docker stack deploy`.
