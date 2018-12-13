#!/bin/bash

function wait_for_pod_running {
  wait_for "kubectl get all --all-namespaces 2>/dev/null | grep $1 | grep \"1/1\""
  while true; do
    result=$(eval $1)
    if [[ ! -z "$result" ]]; then
      break
    fi
    sleep 1
  done
}

function wait_for_everything_up {
  while true; do
    result=$(kubectl get all --all-namespaces 2>/dev/null | grep "0/1")
    if [[ ! -z "$result" ]]; then
      break
    fi
    sleep 1
  done
}

#

case $1 in
  pod-running)
    wait_for_pod_running $2
    ;;
  all-running)
    wait_for_everything_up
    ;;
  *)
    echo $"Usage: $0 {pod-running|all-running}"
    exit 1
esac
