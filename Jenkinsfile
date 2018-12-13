// Copyright 2018 Docker, Inc. All Rights Reserved

properties([buildDiscarder(logRotator(numToKeepStr: '20'))])

pipeline {
    agent none

    options {
        checkoutToSubdirectory('src/github.com/docker/compose-on-kubernetes')
    }

    stages {
        stage('Build') {
            parallel {
                // stage('Build images') {
                //     agent {
                //         label 'team-local && linux'
                //     }
                //     environment {
                //         GOPATH = pwd()
                //         PATH = "/usr/local/go/bin:${GOPATH}/bin:$PATH"
                //         IMAGE_PREFIX = 'kube-compose-'
                //         COMMIT_SHA1 = "$GIT_COMMIT"
                //     }
                //     steps {
                //         script {
                //             env.TAG = env.TAG_NAME == null ? env.COMMIT_SHA1 : env.TAG_NAME
                //             env.IMAGE_REPOSITORY = env.TAG_NAME ? 'docker' : 'dockereng'
                //             env.IMAGE_REPO_PREFIX = env.IMAGE_REPOSITORY + '/' + env.IMAGE_PREFIX
                //         }
                //         sh 'env | sort'
                //         dir('src/github.com/docker/compose-on-kubernetes') {
                //             ansiColor('xterm') {
                //                 sh "make -f docker.Makefile images"
                //                 sh "mkdir images"
                //                 sh '''
                //                 for image in controller controller-coverage api-server api-server-coverage installer e2e-tests; do
                //                     docker save $IMAGE_REPO_PREFIX$image:$TAG -o images/$IMAGE_PREFIX$image.tar
                //                 done
                //                 '''
                //             }
                //             stash name: 'images', includes: "images/*.tar"
                //             // stash name: 'binaries', includes: "bin/*"
                //         }
                //     }
                //     post {
                //         cleanup {
                //             deleteDir()
                //         }
                //     }
                // }
                stage('Build kind image') {
                    agent {
                        label 'team-local && linux'
                    }
                    environment {
                        GOPATH = pwd()
                        PATH = "/usr/local/go/bin:${GOPATH}/bin:$PATH"
                        IMAGE_PREFIX = 'kube-compose-'
                        COMMIT_SHA1 = "$GIT_COMMIT"
                        CLUSTER_NAME = "$GIT_COMMIT"
                        KUBE_VERSION = '1.11.5'
                    }
                    steps {
                        dir('src/github.com/docker/compose-on-kubernetes/scripts/kind-helper') {
                            sh 'docker build . -t kind-helper'
                        }
                        dir('src/github.com/docker/compose-on-kubernetes') {
                            sh 'docker save kind-helper -o kind-helper.tar'
                            stash name: 'kind-helper', includes: "kind-helper.tar"
                        }
                    }
                    post {
                        cleanup {
                            deleteDir()
                        }
                    }
                }
            }
        }
        stage('Tests') {
            parallel {
                stage('Test e2e kube 1.11') {
                    agent {
                        label 'team-local && linux'
                    }
                    environment {
                        GOPATH = pwd()
                        PATH = "/usr/local/go/bin:${GOPATH}/bin:$PATH"
                        IMAGE_PREFIX = 'kube-compose-'
                        COMMIT_SHA1 = "$GIT_COMMIT"
                        CLUSTER_NAME = "$GIT_COMMIT"
                        KUBE_VERSION = '1.11.5'
                    }
                    steps {
                        script {
                            env.TAG = env.TAG_NAME == null ? env.COMMIT_SHA1 : env.TAG_NAME
                            env.IMAGE_REPOSITORY = env.TAG_NAME ? 'docker' : 'dockereng'
                            env.IMAGE_REPO_PREFIX = env.IMAGE_REPOSITORY + '/' + env.IMAGE_PREFIX
                        }
                        dir('src/github.com/docker/compose-on-kubernetes') {
                            unstash('kind-helper')
                            sh 'docker load -i kind-helper.tar'
                            sh 'mkdir -p .kube'
                            sh '''
                            docker run --rm \
                                -e CLUSTER_NAME=$CLUSTER_NAME \
                                -e USER_ID=$(id -u) \
                                -e GROUP_ID=$(id -g) \
                                -v $(pwd)/.kube:/root/.kube \
                                -v /var/run/docker.sock:/var/run/docker.sock \
                                kind-helper sh /entrypoint.sh create
                            '''
                            sh '''
                            docker run --rm \
                                --network=host \
                                -e CLUSTER_NAME=$CLUSTER_NAME \
                                -e USER_ID=$(id -u) \
                                -e GROUP_ID=$(id -g) \
                                -v $(pwd)/.kube:/root/.kube \
                                -v /var/run/docker.sock:/var/run/docker.sock \
                                kind-helper sh /entrypoint.sh kube-init
                            '''
                            sh '''
                            docker run --rm \
                                --network=host \
                                -e CLUSTER_NAME=$CLUSTER_NAME \
                                -e USER_ID=$(id -u) \
                                -e GROUP_ID=$(id -g) \
                                -v $(pwd)/.kube:/root/.kube \
                                -v /var/run/docker.sock:/var/run/docker.sock \
                                kind-helper sh /entrypoint.sh helm-init
                            '''
                            sh '''
                            make e2e-no-provisioning
                            '''
                        }
                        // dir('src/github.com/docker/compose-on-kubernetes') {
                        //     sh 'kind create cluster --name=$CLUSTER_NAME --image=jdrouet/kindest-node:$KUBE_VERSION'
                        //     script {
                        //         env.KUBECONFIG = sh(
                        //             script: 'kind get kubeconfig-path --name=$CLUSTER_NAME',
                        //             returnStdout: true,
                        //         )
                        //         env.USER_ID = sh(
                        //             script: 'id -u',
                        //             returnStdout: true,
                        //         )
                        //         env.GROUP_ID = sh(
                        //             script: 'id -u',
                        //             returnStdout: true,
                        //         )
                        //     }
                        //     // if you have a better idea, you're welcome
                        //     sh 'docker run --rm -it -v $HOME/.kube:/root/.kube -w /root/.kube ubuntu chown -R $USER_ID:$GROUP_ID /root/.kube'
                        //     //
                        //     sh 'kubectl cluster-info'
                        //     //
                        //     sh 'kubectl create namespace tiller'
                        //     sh 'kubectl -n kube-system create serviceaccount tiller'
                        //     sh 'kubectl -n kube-system create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount kube-system:tiller'
                        // }
                    }
                    post {
                        cleanup {
                            sh '''
                            docker run --rm \
                                --network=host \
                                -e CLUSTER_NAME=$CLUSTER_NAME \
                                -e USER_ID=$(id -u) \
                                -e GROUP_ID=$(id -g) \
                                -v $(pwd)/.kube:/root/.kube \
                                -v /var/run/docker.sock:/var/run/docker.sock \
                                kind-helper sh /entrypoint.sh clean
                            '''
                            deleteDir()
                        }
                    }
                }
            }
        }
    }
}
