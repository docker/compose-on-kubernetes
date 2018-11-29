#!/usr/bin/env bash

set -xe

USER="${CIRCLE_PROJECT_USERNAME-${USER-local}}"
VERSION_ID="${CIRCLE_TAG:-${CIRCLE_SHA1}}"
VERSION_ID="$(echo ${VERSION_ID} | cut -c1-10)"
TEST_CLUSTER_NAME="testkit-compose-on-kube-${KUBE_ENV_NAME:-${KUBE_VERSION//./}}-${VERSION_ID//./-}-${USER}"
CLUSTER_NODES=2
KUBE_CONFIG="$HOME/.kube/${KUBE_ENV_NAME:-${KUBE_VERSION}}/config"
TESTKIT_CMD=$(which testkit || echo $HOME/testkit)

auth() {
  test -e "$HOME/.testkit" || {
    mkdir -p "$HOME/.testkit"
    echo "$testkit_pem" | base64 -d > "$HOME/.testkit/testkit.pem"
  }
  test -e "$HOME/.aws" || {
    mkdir -p "$HOME/.aws"
    cat > "$HOME/.aws/credentials" <<EOF
[default]
aws_secret_access_key = $aws_secret_access_key
aws_access_key_id = $aws_access_key_id
EOF
  }
  testkit --driver aws ls
}

testkit() {
    $TESTKIT_CMD "$@"
}

create_cluster() {
    mkdir -p "$(dirname $KUBE_CONFIG)"
    testkit create ubuntu_16.04="$CLUSTER_NODES" --kube --kube-version=${KUBE_VERSION} --kube-config "$KUBE_CONFIG.testkit" --driver aws --name "$TEST_CLUSTER_NAME" --debug
    kubectl --kubeconfig "$KUBE_CONFIG.testkit" config use-context "$TEST_CLUSTER_NAME"
}

delete_cluster() {
  testkit rm --driver aws "$TEST_CLUSTER_NAME" --debug
}

docker() {
  testkit env --driver aws "$TEST_CLUSTER_NAME" -- "$@"

}

case $1 in
  auth)
    auth
    ;;
  create)
    auth
    create_cluster
    ;;
  delete)
    auth
    delete_cluster
    ;;
  docker)
    auth
    docker "$@"
    ;;
  *)
    echo $"Usage: $0 {create|wait|delete}"
    exit 1
esac
