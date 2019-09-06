# Install on Minikube

## Pre-requisites
- [Minikube](https://github.com/kubernetes/minikube)
- To install etcd using these instructions, you must have [Helm](https://helm.sh) in your client environment.
- [Download the Compose on Kubernetes installer](https://github.com/docker/compose-on-kubernetes/releases).

## Create compose namespace

- First you should have the confirmation that everything is running and `kubectl` is pointing to the minikube vm by running `minikube status`.
If not, please start it by running `minikube start`. That should create and/or start Minikube's VM.
- Then, create a compose namespace by running `kubectl create namespace compose`.

## Deploy etcd

Compose on Kubernetes requires an etcd instance (in addition to the kube-system etcd instance). Please follow [How to deploy etcd](./deploy-etcd.md).

## Deploy Compose on Kubernetes

Run `installer-[darwin|linux|windows.exe] -namespace=compose -etcd-servers=http://compose-etcd-client:2379`.

## Deploy a stack in the cluster

By now you should be able to [Check that Compose on Kubernetes is installed](../README.md#check-that-compose-on-kubernetes-is-installed) and [Deploy a stack](../README.md#deploy-a-stack).

Then when listing Minikube's services with `minikube service list` you should see something like:
```sh
$ minikube service list
|-------------|-----------------------|-----------------------------|
|  NAMESPACE  |         NAME          |             URL             |
|-------------|-----------------------|-----------------------------|
| compose     | compose-api           | No node port                |
| compose     | compose-etcd          | No node port                |
| compose     | compose-etcd-client   | No node port                |
| compose     | etcd-restore-operator | No node port                |
| default     | db                    | No node port                |
| default     | kubernetes            | No node port                |
| default     | web                   | No node port                |
| default     | web-published         | http://192.168.99.103:32399 |
| default     | words                 | No node port                |
| kube-system | kube-dns              | No node port                |
| kube-system | kubernetes-dashboard  | No node port                |
| kube-system | tiller-deploy         | No node port                |
|-------------|-----------------------|-----------------------------|
```

To access our example web application, please run `minikube service web-published`. This should open a browser pointing to Minikube's VM ip in the service's exposed port. In this case 32399.
