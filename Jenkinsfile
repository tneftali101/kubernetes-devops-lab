
pipeline {
    agent {
        kubernetes {
            yaml '''
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: jenkins-deployer
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000

  containers:
    - name: curl
      image: curlimages/curl:8.17.0
      command:
        - sleep
      args:
        - "99d"
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 200m
          memory: 128Mi

    - name: buildkit
      image: moby/buildkit:v0.35.0-rootless
      command:
        - sleep
      args:
        - "99d"
      env:
        - name: BUILDKITD_FLAGS
          value: "--oci-worker-no-process-sandbox"
      securityContext:
        seccompProfile:
          type: Unconfined
        appArmorProfile:
          type: Unconfined
      resources:
        requests:
          cpu: 250m
          memory: 512Mi
        limits:
          cpu: "2"
          memory: 2Gi
      volumeMounts:
        - name: buildkit-data
          mountPath: /home/user/.local/share/buildkit

    - name: kubectl
      image: alpine/kubectl:1.36.4
      command:
        - sleep
      args:
        - "99d"
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 200m
          memory: 128Mi

  volumes:
    - name: buildkit-data
      emptyDir: {}
'''
        }
    }

    options {
        disableConcurrentBuilds()
    }

    environment {
        IMAGE_REPOSITORY = 'ghcr.io/tneftali101/nginx-lab'
        DEPLOY_NAMESPACE = 'monitoring-lab'
    }

    stages {
        stage('Verify Source') {
            steps {
                echo 'Checking application source files...'

                sh '''
                    pwd
                    ls -l app/
                    test -f app/Dockerfile
                    test -f app/index.html
                '''

                echo 'Application files verified!'
            }
        }

        stage('Build and Push Image') {
            steps {
                echo 'Building and publishing NGINX image...'

                container('buildkit') {
                    withCredentials([
                        usernamePassword(
                            credentialsId: 'ghcr-publisher',
                            usernameVariable: 'GHCR_USER',
                            passwordVariable: 'GHCR_TOKEN'
                        )
                    ]) {
                        sh '''
                            set +x
                            set -eu
                            umask 077

                            export DOCKER_CONFIG="$(mktemp -d)"
                            trap 'rm -rf "$DOCKER_CONFIG"' EXIT

                            AUTH="$(printf '%s:%s' \
                              "$GHCR_USER" "$GHCR_TOKEN" \
                              | base64 | tr -d '\\n')"

                            printf '{"auths":{"ghcr.io":{"auth":"%s"}}}\\n' \
                              "$AUTH" > "$DOCKER_CONFIG/config.json"

                            IMAGE="$IMAGE_REPOSITORY:$GIT_COMMIT"

                            echo "Publishing image: $IMAGE"

                            buildctl-daemonless.sh build \
                              --frontend dockerfile.v0 \
                              --local context=app \
                              --local dockerfile=app \
                              --output "type=image,name=$IMAGE,push=true"

                            echo 'Image published successfully!'
                        '''
                    }
                }
            }
        }

        stage('Deploy to Kubernetes') {
            steps {
                echo 'Deploying new image to Kubernetes...'

                container('kubectl') {
                    sh '''
                        set -eu

                        IMAGE="$IMAGE_REPOSITORY:$GIT_COMMIT"

                        kubectl set image deployment nginx-web \
                          nginx="$IMAGE" \
                          -n "$DEPLOY_NAMESPACE"

                        echo "Requested deployment of $IMAGE"
                    '''
                }
            }
        }

        stage('Verify Deployment') {
            steps {
                echo 'Checking Kubernetes rollout...'

                container('kubectl') {
                    sh '''
                        kubectl rollout status deployment nginx-web \
                          -n "$DEPLOY_NAMESPACE" \
                          --timeout=180s
                    '''
                }

                echo 'Kubernetes rollout completed!'
            }
        }

        stage('Test NGINX') {
            steps {
                echo 'Verifying deployed application version...'

                container('curl') {
                    sh '''
                        set -eu

                        EXPECTED_LINE="$(grep -o \
                          '<h2>Application Version [^<]*</h2>' \
                          app/index.html | head -n 1)"

                        test -n "$EXPECTED_LINE"

                        echo "Expected: $EXPECTED_LINE"

                        RESPONSE_FILE="$(mktemp)"
                        trap 'rm -f "$RESPONSE_FILE"' EXIT

                        for i in 1 2 3 4 5; do
                            echo "Starting HTTP version check $i..."

                            curl --fail --silent --show-error \
                              --retry 5 \
                              --retry-all-errors \
                              --retry-delay 3 \
                              --connect-timeout 5 \
                              --max-time 15 \
                              --output "$RESPONSE_FILE" \
                              http://nginx-service.monitoring-lab.svc.cluster.local./

                            if ! grep -Fq "$EXPECTED_LINE" "$RESPONSE_FILE"; then
                                echo "ERROR: Deployed version does not match source!"
                                exit 1
                            fi

                            echo "HTTP version check $i passed"
                        done
                    '''
                }

                echo 'Application version verified successfully!'
            }
        }
    }
}
