# Stack to Kubernetes mapping

There are several key differences between Swarm and Kubernetes which prevent a
1:1 mapping of Swarm onto Kubernetes. An opinionated mapping can be achieved,
however, with a couple of minor caveats.

As a stack is essentially just a list of Swarm services the mapping is done on a
per service basis.

## Swarm service to Kubernetes objects

There are fundamentally two classes of Kubernetes objects required to map a
Swarm service: Something to deploy and scale the containers and something to
handle intra- and extra-stack networking.

### Pod deployment

In Kubernetes one does not manipulate individual containers but rather a set of
containers called a
[_pod_](https://kubernetes.io/docs/concepts/workloads/pods/pod/). Pods can be
deployed and scaled using different controllers depending on what the desired
behaviour is.

The following Compose snippet declares a global service:

```yaml
version: "3.6"

services:
  worker:
    image: dockersamples/examplevotingapp_worker
    deploy:
      mode: global
```

If a service is declared to be global, Compose on Kubernetes uses a
[_DaemonSet_](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/)
to deploy pods. **Note:** Such services cannot use a persistent volume.

The following Compose snippet declares a service that uses a volume for storage:

```yaml
version: "3.6"

services:
  mysql:
    volumes:
      - db-data:/var/lib/mysql

volumes:
  db-data:
```

If a service uses a volume, a
[_StatefulSet_](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
is used. For more information about how volumes are handled, see the
[following section](#volumes).

In all other cases, a
[_Deployment_](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
is used.

#### Volumes

There are several different types of volumes that are handled by Compose for
Kubernetes.

The following Compose snippet declares a service that uses a persistent volume:

```yaml
version: "3.6"

services:
  mysql:
    volumes:
      - db-data:/var/lib/mysql

volumes:
  db-data:
```

A
[_PersistentVolumeClaim_](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
with an empty provider is created when one specifies a persistent volume for a
service. This requires that Kubernetes has a default storage provider
configured.

The following Compose snippet declares a service with a host bind mount:

```yaml
version: "3.6"

services:
  web:
    image: nginx:alpine
    volumes:
      - type: bind
        source: /srv/data/static
        target: /opt/app/static
```

Host bind mounts are supported but note that an absolute source path is
required.

The following Compose snippet declares a service with a tmpfs mount:

```yaml
version: "3.6"

services:
  web:
    image: nginx:alpine
    tmpfs:
      - /tmpfs
```

Mounts of type tmpfs create an empty directory which is stored in memory for the
life of the pod.

#### Secrets

Secrets in Swarm are a simple key-value pair. In Kubernetes, a secret has a name
and then a map of keys to values. This means that the mapping is non-trivial.
The following Compose snippet shows a service with two secrets:

```yaml
version: "3.6"

services:
  web:
    image: nginx:alpine
    secrets:
      - mysecret
      - myexternalsecret

secrets:
  mysecret:
    file: ./my_secret.txt
  myexternalsecret:
    external: true
```

When deployed using the Docker CLI, a Kubernetes secret will be created from the
client-local file `./my_secret.txt`. The secret's name will be `my_secret`, it
will have a single key `my_secret.txt` whose value will be the contents of the
file. As expected, this secret will then be mounted to `/run/secrets/my_secret`
in the pod.

External secrets need to be created manually by the user using `kubectl` or the
relevant Kubernetes APIs. The secret name must match the Swarm secret name, its
key must be `file` and the associated value the secret value:

```bash
$ echo -n 'external secret' > ./file
$ kubectl create secret generic myexternalsecret --from-file=./file
secret "myexternalsecret" created
```

#### Configs

Configs work the same as [secrets](#secrets).

### Intra-stack networking

Kubernetes does not have the notion of a _network_ like Swarm does. Instead all
pods that exist in a namespace can network with each other. In order for DNS
name resolution between pods to work, a
[_HeadlessService_](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services)
is required.

As we are unable to determine which stack services need to communicate with each
other in advance, we create a _HeadlessService_ for each stack service with the
name of the service.

**Note:** Service names must be unique by Kubernetes namespace.

### Extra-stack networking

In order for stack services to be accessible to the outside word, a port must be
exposed as is shown in the following snippet:

```yaml
version: "3.6"

services:
  web:
    image: nginx:alpine
    ports:
    - target: 80
      published: 8080
      protocol: tcp
      mode: host
```

For this case, a published port of 8080 is specified for the target port of 80.
To do this, a
[_LoadBalancer_](https://kubernetes.io/docs/concepts/services-networking/service/#type-loadbalancer)
service is created with the service name suffixed by `-published`. This
implicitly creates
[_NodePort_](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport)
and
_ClusterIP_ services.

**Note**: For clusters that do not have a _LoadBalancer_, the controller can be
run with `--default-service-type=NodePort`. This way, a _NodePort_ service is
created instead of a _LoadBalancer_ service, with the published port being used
as the node port. This requires that the published port to be within the
configured _NodePort_ range.

If only a target port is specified then a _NodePort_ is created with a random
port.
