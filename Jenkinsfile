// Copyright 2018 Docker, Inc. All Rights Reserved

properties([buildDiscarder(logRotator(numToKeepStr: '20'))])

pipeline {
    agent {
        label 'team-local && pipeline'
    }

    options {
        skipDefaultCheckout(true)
    }

    stages {
        stage('Build') {
            agent {
                label 'team-local && linux'
            }
            steps  {
                dir('src/github.com/docker/compose-on-kubernetes') {
                    sh 'printenv'
                    ansiColor('xterm') {
                        sh 'docker ps'
                    }
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
