## Deploy etcd

### Install Helm server side component

- Create the tiller service account: `kubectl -n kube-system create serviceaccount tiller`.
- Give it admin access to your cluster (note: you might want to reduce the scope of this): `kubectl -n kube-system create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount 
kube-system:tiller`.
- Run `helm init --service-account tiller` to initialize the helm component.

### Deploy etcd operator and create an etcd cluster

- Make sure the `compose` namespace exists on your cluster.
- Run `helm install --name etcd-operator stable/etcd-operator --namespace compose` to install the etcd-operator chart.
- Create an etcd cluster definition like this one in a file named compose-etcd.yaml:

```yaml
apiVersion: "etcd.database.coreos.com/v1beta2"
kind: "EtcdCluster"
metadata:
  name: "compose-etcd-client"
  namespace: "compose"
spec:
  size: 3
  version: "3.2.13"
```
- Run `kubectl apply -f compose-etcd.yaml`.
- This should bring an etcd cluster in the `compose` namespace.

**Note: this cluster configuration is really naive and does does not use mutual TLS to authenticate application accessing the data. For enabling mutual TLS, please refer to https://github.com/coreos/etcd-operator** 
