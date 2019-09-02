# Install on Amazon EKS

## Pre-requisites
- To install Compose on Kubernetes on Amazon EKS, you must create an EKS cluster.
- To install etcd using these instructions, you must have [Helm](https://helm.sh) in your client environment.
- [Download the Compose on Kubernetes installer](https://github.com/docker/compose-on-kubernetes/releases).

## Create compose namespace

Just run `kubectl create namespace compose`.

## Deploy etcd

Compose on Kubernetes requires an etcd instance (in addition to the kube-system etcd instance). Please follow [How to deploy etcd](./deploy-etcd.md).

## Deploy Compose on Kubernetes

Run `installer-[darwin|linux|windows.exe] -namespace=compose -etcd-servers=http://compose-etcd-client:2379 -tag=v0.4.23`.

**Note: To setup Mutual TLS with the etcd instance, you can use `etcd-ca-file`, `etcd-key-file` and `etcd-cert-file` flags.**

## Deploy a stack in the cluster

By now you should be able to [Check that Compose on Kubernetes is installed](../README.md#check-that-compose-on-kubernetes-is-installed) and [Deploy a stack](../README.md#deploy-a-stack).

Then when listing resources with `kubectl get svc` you should see something like:
``` 
NAME            TYPE           CLUSTER-IP       EXTERNAL-IP                                                                  PORT(S)           AGE
db              ClusterIP      None             <none>                                                                       55555/TCP         4m51s
kubernetes      ClusterIP      10.100.0.1       <none>                                                                       443/TCP           27m
web             ClusterIP      None             <none>                                                                       55555/TCP         4m51s
web-published   LoadBalancer   10.100.130.153   ad0884309cd8a11e98ccc0246f5f7bb0-1039555521.eu-central-1.elb.amazonaws.com   33000:30123/TCP   4m51s
words           ClusterIP      None             <none>                                                                       55555/TCP         4m51s
```

To access our example web application, open a browser and go to `<LoadBalancer external-ip>:33000`.

