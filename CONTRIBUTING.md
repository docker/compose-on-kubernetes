# Contribution guide

## Branches

Development & maintenance should be done on master
Everything that must be backported to Pinata beta 1 / ucp 18.01 should be cherry-picked into v0.1.X (process below)

## Cherry-picking

When a PR should be back ported to v0.1.X, we need to mark it with the label `process/cherry-pick`
Once this PR is ready to merge (or after it has been merged), a maintainer creates a new PR with title `[v0.1.X] Cherry-picking #(ID of main PR)` that contains cherry-picked commits of the main PR
In the comment of this cherry-picking PR, we add a link to the main PR
When a maintainer merges a cherry-picking PR, he adds the `process/cherry-picked` to the main PR

This way it is easy to list merged PRs that require cherry-picking

## Dev with controller and API Server

We moved from a CRD to a custom API Server. This has some implication on the development workflow and on the whole logic.

### What does the API server do

Instead of storing our Stack resources in a CRD within the central API Server, we now have our own API server that will handle all our resources (including serving REST APIs and storing entities). This API server is then aggregated by the central
API server

So now, we use our own ETCD server as well, as a backing store. In UCP, we'll use existing ETCD installation (the same that holds UCP/Enzi data and kubernetes resources). In dev mode, we'll run an unsafe ETCD instance in a container.
We yet need to figure plans for Pinata (reuse Pinata/k8s etcd ?)

### How to if I don't need to develop within the API server (only working on the stack support within the controller)

The dev flow is quite the same. You just need to run `make setup-api-server-dev && make run-api-server-dev` from a terminal before running/debugging the controller.

### How to debug the API server

Do as previous, then stop hit `ctrl-c` to stop the API-Server. Setup your debugger to run the API server with the following arguments: 
`./bin/api-server --etcd-servers=http://127.0.0.1:7777 --kubeconfig ~/.kube/config --authentication-kubeconfig ~/.kube/config --authorization-kubeconfig ~/.kube/config --secure-port 9443`