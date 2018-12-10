# Install on Azure AKS

## Pre-requisites
- To install Compose on Kubernetes on Azure AKS, you must create an AKS cluster with RBAC enabled.
- To install etcd using those instructions, you must have Helm on your client environment
- Download the Compose on Kubernetes installer

## Create compose namespace

Just run `kubectl create namespace compose`

## Deploy etcd

If you already have an etcd instance, skip this.

### Install Helm server side component

- Create the tiller service account: `kubectl -n kube-system create serviceaccount tiller`
- Give it admin access to your cluster (note: you might want to reduce the scope of this): `kubectl -n kube-system create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount kube-system:tiller`
- Run `helm init --service-account tiller` to initialize the helm component.

### Deploy etcd operator and create an etcd cluster

- Run `helm install --name etcd-operator stable/etcd-operator --namespace compose` to install the etcd-operator chart
- Create an etcd cluster definition like this one in a file named compose-etcd.yaml:

```
apiVersion: "etcd.database.coreos.com/v1beta2"
kind: "EtcdCluster"
metadata:
  name: "compose-etcd"
  namespace: "compose"
spec:
  size: 3
  version: "3.2.13"
```
- Run `kubectl apply -f compose-etcd.yaml`
- This should bring an etcd cluster in the compose namespace

**Note: this cluster configuration is really naive and does does not use mutual TLS to authenticate application accessing the data. For enabling mutual TLS, please refer to https://github.com/coreos/etcd-operator** 

## Deploy Compose on Kubernetes

run `installer-[darwin|linux|windows.exe] -namespace=compose -etcd-servers=http://compose-etcd-client:2379 -tag=v0.4.16 -skip-liveness-probes=true`

**Note: There is currently an issue with AKS where the API Server Liveness Probe always failes due to a TLS issue. We'll get in touch with Microsoft to try to solve the issue in the future.**

**Note: To setup Mutual TLS with the etcd instance, you can use `etcd-ca-file`, `etcd-key-file` and `etcd-cert-file` flags.**