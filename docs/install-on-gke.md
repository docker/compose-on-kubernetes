# Install on GKE

## Pre-requisites
- To install Compose on Kubernetes on GKE, you must create a Kubernetes cluster.
- To install etcd using these instructions, you must have [Helm](https://helm.sh) in your client environment.
- [Download the Compose on Kubernetes installer](https://github.com/docker/compose-on-kubernetes/releases).

## Create compose namespace

Just run `kubectl create namespace compose`.

## Deploy etcd

If you already have an etcd instance, skip this.
Otherwise, please follow [How to deploy etcd](./deploy-etcd.md).

## Admin rights

You must grant your current Google identity the `cluster-admin` Role:

```console
$ gcloud info | grep Account
Account: [ACCOUNT]

$ kubectl create clusterrolebinding <ACCOUNT>-cluster-admin-binding --clusterrole=cluster-admin --user=<ACOUNT>
clusterrolebinding.rbac.authorization.k8s.io "<ACCOUNT>-cluster-admin-binding" created
```

## Deploy Compose on Kubernetes

Run `installer-[darwin|linux|windows.exe] -namespace=compose -etcd-servers=http://compose-etcd-client:2379 -tag=v0.4.18`.

**Note: To setup Mutual TLS with the etcd instance, you can use `etcd-ca-file`, `etcd-key-file` and `etcd-cert-file` flags.**
