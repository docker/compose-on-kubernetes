#!/bin/sh
set -e
if ! curl ${ETCD_ADDRESS} 1> /dev/null 2>/dev/null; then
    EXPOSE=$(echo ${ETCD_ADDRESS} | sed "s|https*://||")
    docker start compose_dev_etcd || docker run --name compose_dev_etcd -d -p ${EXPOSE}:2379 quay.io/coreos/etcd:v3.2.9 //usr/local/bin/etcd -advertise-client-urls=http://0.0.0.0:2379 -listen-client-urls=http://0.0.0.0:2379
fi
./bin/api-server --validator bin/validate-yaml --etcd-servers=${ETCD_ADDRESS} --kubeconfig ~/.kube/config --authentication-kubeconfig ~/.kube/config --authorization-kubeconfig ~/.kube/config --secure-port 9443
