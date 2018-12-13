#!/usr/bin/env bash

set -xe

##

clean_kube() {
  chmod 666 /root/.kube
  chown -R $USER_ID:$GROUP_ID /root/.kube
}

clean_helm() {
  chmod 666 /root/.helm
  chown -R $USER_ID:$GROUP_ID /root/.helm
}

create_cluster() {
  docker rm -f kind-$CLUSTER_NAME-control-plane || true
  kind create cluster --name=$CLUSTER_NAME --image=jdrouet/kindest-node:$KUBE_VERSION
  export KUBECONFIG=$(kind get kubeconfig-path --name=$CLUSTER_NAME)
  chmod 666 /root/.kube
}

kubectl_cmd() {
  shift
  export KUBECONFIG=$(kind get kubeconfig-path --name=$CLUSTER_NAME)
  kubectl "$@"
  clean_kube
}

kube_init() {
  export KUBECONFIG=$(kind get kubeconfig-path --name=$CLUSTER_NAME)
  kubectl create namespace tiller
  kubectl -n kube-system create serviceaccount tiller
  kubectl -n kube-system create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount kube-system:tiller
  clean_kube
}

helm_init() {
  export KUBECONFIG=$(kind get kubeconfig-path --name=$CLUSTER_NAME)
  helm init --wait --service-account tiller
  # sh /wait_for.sh pod-running "pod/etcd-kind-$CLUSTER_NAME-control-plane"
  # sh /wait_for.sh pod-running "pod/tiller-deploy"
  clean_helm
}

case $1 in
  create)
    create_cluster
    ;;
  kube-init)
    kube_init
    ;;
  helm-init)
    helm_init
    ;;
  kubectl)
    kubectl_cmd "$@"
    ;;
  clean)
    clean_helm
    clean_kube
    ;;
  *)
    echo $"Usage: $0 {create|kubectl|clean}"
    exit 1
esac
