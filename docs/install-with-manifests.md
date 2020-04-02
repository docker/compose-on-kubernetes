# Install using manifests

## Pre-requisites

- Depends on the type of cluster
- You'll need the etcd-operator helm chart installed

## Deploy etcd

Compose on Kubernetes requires an etcd instance (in addition to the kube-system etcd instance). Please follow [How to deploy etcd](./deploy-etcd.md).

## Deploy Compose on Kubernetes

- Substitute `${namespace}` in all the files with whichever templating method you prefer.
- Run `kubectl apply -R -f ./docs/manifests/` from the root of this repository.

**Note: To setup Mutual TLS with the etcd instance, you can use `etcd-ca-file`, `etcd-key-file` and `etcd-cert-file` flags.**
