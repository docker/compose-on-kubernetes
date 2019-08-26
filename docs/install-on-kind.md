# Install on Kind (Kubernetes in Docker)

## Pre-requisites
- [Kind](https://kind.sigs.k8s.io/ 0.5 or later
- To install etcd using these instructions, you must have [Helm](https://helm.sh) in your client environment.
- [Download the Compose on Kubernetes installer](https://github.com/docker/compose-on-kubernetes/releases).
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) availabe in path.

## Create compose namespace

Create a Kind cluster with `kind create cluster`

## Populate Kubernetes Config

Configure kubectl so it points to your freshly created Kind cluster
`export KUBECONFIG="$(kind get kubeconfig-path)"`

## Deploy etcd

Compose on Kubernetes requires an etcd instance (in addition to the kube-system etcd instance). Please follow [How to deploy etcd](./deploy-etcd.md).

## Deploy Compose on Kubernetes

Run `./installer-[darwin|linux|windows.exe] -namespace=compose -etcd-servers=http://compose-etcd-client:2379`.

## Deploy a stack in the cluster

By now you should be able to [Check that Compose on Kubernetes is installed](../README.md#check-that-compose-on-kubernetes-is-installed):

```bash
$ kubectl api-versions | grep compose
compose.docker.com/v1beta1
compose.docker.com/v1beta2
```

If the APIs are visible it should be possible to [Deploy a stack](../README.md#deploy-a-stack).

If everything has worked the `web-published` service should be visible from `kubectl` e.g:

```
$ kubectl get services
NAME            TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)           AGE
db              ClusterIP      None             <none>        55555/TCP         3m31s
kubernetes      ClusterIP      10.152.183.1     <none>        443/TCP           33m
web             ClusterIP      None             <none>        55555/TCP         3m31s
web-published   LoadBalancer   10.152.183.180   <pending>     33000:30218/TCP   3m31s
words           ClusterIP      None             <none>        55555/TCP         3m31s
```

In this case it's possible to open <http://10.152.183.180:33000/> in the browser, but your Cluster IP will most likely be different.