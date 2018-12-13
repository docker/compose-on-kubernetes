FROM golang:1.10.5-stretch

ENV USER_ID 1000
ENV GROUP_ID 1000

RUN mkdir /storage

RUN go get sigs.k8s.io/kind && \
  go get github.com/docker/cli/cmd/docker

RUN apt-get update && \
  apt-get install -y apt-transport-https && \
  curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
  echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list && \
  apt-get update && \
  apt-get install -y kubectl && \
  rm -rf /var/lib/apt/lists/*

RUN curl https://raw.githubusercontent.com/helm/helm/master/scripts/get | bash

ENV CLUSTER_NAME testing
ENV KUBE_VERSION 1.11.5

COPY wait_for.sh /wait_for.sh
COPY entrypoint.sh /entrypoint.sh

CMD ["sh", "/entrypoint.sh"]
