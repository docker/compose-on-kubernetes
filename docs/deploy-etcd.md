## Deploy etcd

### Install Helm server side component

`etcd` is deployed using Helm so we must first install the Helm server:

- Create the tiller service account: `kubectl -n kube-system create serviceaccount tiller`.
- Give it admin access to your cluster (note: you might want to reduce the scope of this): `kubectl -n kube-system create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount kube-system:tiller`.
- Run `helm init --service-account tiller` to initialize the helm component.
- Run `kubectl get pods --namespace kube-system` and check that the tiller-deploy container was created and is in running state
```
NAME                            READY   STATUS    RESTARTS   AGE
tiller-deploy-5d6cc99fc-qdv4q   1/1     Running   0          8s
```

### Deploy etcd operator

- Make sure the `compose` namespace exists on your cluster.
- Run `helm install --name etcd-operator stable/etcd-operator --namespace compose` to install the etcd-operator chart.
- Run `kubectl get pods --namespace compose` and check that etcd-operator containers were created and are in running state.
```
NAME                                                              READY   STATUS    RESTARTS   AGE
etcd-operator-etcd-operator-etcd-backup-operator-ddd46947d4twzb   1/1     Running   0          22m
etcd-operator-etcd-operator-etcd-operator-5db4855dd8-8hh2t        1/1     Running   0          22m
etcd-operator-etcd-operator-etcd-restore-operator-75d7744cl7chc   1/1     Running   0          22m
```

### Option 1: Create an etcd cluster (for quick evaluation)

This will create an etcd cluster quickly, but without High Availability, or persistent storage, and that can be accessed without authentication. This implies that if all pods in the cluster are scheduled on the same Kubernetes node, if the node is shut down or restarted, it will not be able to recover.
- Write an etcd cluster definition like this one in a file named compose-etcd.yaml:

```yaml
apiVersion: "etcd.database.coreos.com/v1beta2"
kind: "EtcdCluster"
metadata:
  name: "compose-etcd"
  namespace: "compose"
spec:
  size: 3
  version: "3.3.15"
  pod:
    affinity:
      podAntiAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchExpressions:
              - key: etcd_cluster
                operator: In
                values:
                - compose-etcd
            topologyKey: kubernetes.io/hostname
```
- Run `kubectl apply -f compose-etcd.yaml`.
- This should bring an etcd cluster in the `compose` namespace.
- Run `kubectl get pods --namespace compose` and check that containers are in running state.
```
NAME                                                              READY   STATUS    RESTARTS   AGE
compose-etcd-5gk95j4ms6                                           1/1     Running   0          21m
compose-etcd-nqmcwk4gdf                                           1/1     Running   0          21m
compose-etcd-sxplrdthp6                                           1/1     Running   0          20m
```

**Note: this cluster configuration is really naive and does does not use mutual TLS to authenticate application accessing the data. For enabling mutual TLS, please refer to https://github.com/coreos/etcd-operator**

### Option 2: Create a secure and highly available etcd cluster

This requires a slightly more advanced template, and some tooling for generating TLS credentials.
We will start with the same YAML as in option 1. Then we will add some options to make it more robust
- First, enable persistent storage. To do this, follow [Custom PersistentVolumeClaim definition](https://github.com/coreos/etcd-operator/blob/master/doc/user/spec_examples.md#custom-persistentvolumeclaim-definition).
  - To list the persistent storage classes available in your cluster, run `kubectl get storageclass`
- If you have enough nodes in your cluter, you can use a more restricting antiafinity rule, enforcing that each etcd pod will [run on a different Kubernetes node](https://github.com/coreos/etcd-operator/blob/master/doc/user/spec_examples.md#three-member-cluster-with-node-selector-and-anti-affinity-across-nodes)
  - Don't forget to replace `$cluster_name` in those samples with `compose-etcd`
- Finaly, setup mutual TLS
  - Follow https://coreos.com/os/docs/latest/generate-self-signed-certificates.html to generate all the TLS material required. Server certificate hosts must contain `compose-etcd.compose.svc`.
  - Follow https://github.com/coreos/etcd-operator/blob/master/doc/user/cluster_tls.md#static-cluster-tls-policy to generate the required secrets, and modify the cluster spec
  - When installing the Compose on Kubernetes components, pass the generated client CA, Cert and Key to Compose on Kubernetes installer using flags `etcd-ca-file`, `etcd-cert-file` and `etcd-key-file`
