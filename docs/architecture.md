# Architecture

Compose on Kubernetes is made up of server-side and client-side components.
This architecture was chosen so that we could keep track of stack objects on the server side, and thus allow to manage the whole stack life-cicle instead of having to manage each generated Kubernetes object by hand.

The REST API is made of a custom API server exposed to Kubernetes clients using
[API server aggregation](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#api-server-aggregation).

The client communicates with the server using a REST API. It creates a stack by
either POSTing a serialized stack struct (v1beta2) or a Compose file (v1beta1)
to the REST store component. This REST store component stores the stack in an ETCD database.

The stack stored in ETCD is considered the desired state.
The Compose controller is responsible for cutting the stack up into Kubernetes
components, reconciling the current cluster state with the desired state, and
aggregating the stack status.

Manipulations of the stack are performed by the client using the REST API
provided by the REST store component.

## Server-side architecture

There are two server-side components in Compose on
Kubernetes: the REST store (the aggregated API server), and the Compose
controller.

The REST store extends the Kubernetes API by adding routes for creating and
manipulating stacks. It also stores the stacks in an ETCD database. It also contains the logic to convert v1beta1 representations to v1beta2, so that the Compose controller can work with only one representation of the Stack.

The Compose controller is responsible for converting a stack struct (v1beta2 schema) into Kubernetes objects and then
reconciling the current cluster state with the desired one.

### API server

The API server implementation of the REST store is more complex than a classic CRD, but brings many more possibilities (subresources, non-trivial validation, user identity recording).
Instead of relying on Kubernetes to create the API endpoints and logic for
storing and manipulating stack objects, Compose on Kubernetes deploys a custom
server-side component to do this. As part of the install process, the component
is registered with the Kubernetes API server for API aggregation so that it is
forwarded all requests on the `compose.docker.com/` route.

This means that we have complete control over the routes created, can perform
tasks like validation, but are required to use a custom installer.

### Compose controller

The Compose controller fetches the stack structure from the REST store. This struct is then cut up into Kubernetes
resources which the controller create and manipulate through the Kubernetes
API server. The mapping for this can be found in [mapping.md](./mapping.md).

The API server also records user identity, group membership and claims on stack creations and updates, and expose them as a subresource. The Controller consumes this subresource to impersonate this user, to create Kubernetes objects with this identity.

## Client-side architecture

### v1beta1

The v1beta1 API requires that the client uploads a Compose file to describe the
stack. This was used in early versions of Compose on Kubernetes and is used by
Docker Enterprise 2.0.

In the medium-term, we aim to deprecate this API.

### v1beta2

The v1beta2 API requires that the client parses the Compose file and uploads a
stack struct. This is used by the Docker CLI by default.
