# Install on GKE

## Pre-requisites
- To install Compose on Kubernetes on GKE, you must create a Kubernetes cluster.
- To install etcd using these instructions, you must have [Helm](https://helm.sh) in your client environment.
- [Download the Compose on Kubernetes installer](https://github.com/docker/compose-on-kubernetes/releases).

**Important:** You will need a custom build of Docker CLI to deploy stacks onto GKE. The build must include [this PR](https://github.com/docker/cli/pull/1583) which has been merged onto the master branch.

## Create compose namespace

Just run `kubectl create namespace compose`.

## Deploy etcd

Compose on Kubernetes requires an etcd instance (in addition to the kube-system etcd instance). Please follow [How to deploy etcd](./deploy-etcd.md).

## Admin rights

You must grant your current Google identity the `cluster-admin` Role:

```console
$ gcloud info | grep Account
Account: [ACCOUNT]

$ kubectl create clusterrolebinding <ACCOUNT>-cluster-admin-binding --clusterrole=cluster-admin --user=<ACOUNT>
clusterrolebinding.rbac.authorization.k8s.io "<ACCOUNT>-cluster-admin-binding" created
```

## Deploy Compose on Kubernetes

Run `installer-[darwin|linux|windows.exe] -namespace=compose -etcd-servers=http://compose-etcd-client:2379`.

**Note: To setup Mutual TLS with the etcd instance, you can use `etcd-ca-file`, `etcd-key-file` and `etcd-cert-file` flags.**
