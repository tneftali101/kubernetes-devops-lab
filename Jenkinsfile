
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
                echo 'Saving current image and deploying new version...'

                container('kubectl') {
                    sh '''
                        set -eu

                        PREVIOUS_IMAGE="$(kubectl get deployment nginx-web \
                          -n "$DEPLOY_NAMESPACE" \
                          -o jsonpath='{.spec.template.spec.containers[0].image}')"

                        test -n "$PREVIOUS_IMAGE"

                        printf '%s\\n' "$PREVIOUS_IMAGE" \
                          > .previous-nginx-image

                        echo "Previously deployed image: $PREVIOUS_IMAGE"

                        IMAGE="$IMAGE_REPOSITORY:$GIT_COMMIT"

                        echo "Deploying new image: $IMAGE"

                        kubectl set image deployment nginx-web \
                          nginx="$IMAGE" \
                          -n "$DEPLOY_NAMESPACE"

                        echo 'Kubernetes image update requested!'
                    '''
                }
            }
        }

        stage('Verify Deployment') {
            steps {
                echo 'Checking Kubernetes rollout with rollback protection...'

                container('kubectl') {
                    sh '''
                        set -eu

                        IMAGE="$IMAGE_REPOSITORY:$GIT_COMMIT"
                        PREVIOUS_IMAGE="$(cat .previous-nginx-image)"

                        if kubectl rollout status deployment nginx-web \
                          -n "$DEPLOY_NAMESPACE" \
                          --timeout=180s; then

                            echo 'Kubernetes rollout completed successfully!'

                        else
                            echo 'WARNING: New deployment did not complete.'
                            echo 'Checking whether rollback is safe...'

                            CURRENT_IMAGE="$(kubectl get deployment nginx-web \
                              -n "$DEPLOY_NAMESPACE" \
                              -o jsonpath='{.spec.template.spec.containers[0].image}')"

                            if [ "$CURRENT_IMAGE" != "$IMAGE" ]; then
                                echo 'Deployment image changed unexpectedly.'
                                echo 'Automatic rollback skipped for safety.'
                                exit 1
                            fi

                            if [ "$PREVIOUS_IMAGE" = "$IMAGE" ]; then
                                echo 'Previous image is identical to new image.'
                                echo 'Automatic rollback skipped.'
                                exit 1
                            fi

                            echo "Restoring image: $PREVIOUS_IMAGE"

                            if kubectl set image deployment nginx-web \
                              nginx="$PREVIOUS_IMAGE" \
                              -n "$DEPLOY_NAMESPACE"; then

                                echo 'Rollback requested. Verifying...'

                                if kubectl rollout status deployment nginx-web \
                                  -n "$DEPLOY_NAMESPACE" \
                                  --timeout=180s; then

                                    echo 'Rollback completed successfully!'
                                else
                                    echo 'ERROR: Rollback verification failed!'
                                fi
                            else
                                echo 'ERROR: Rollback could not be requested!'
                            fi

                            echo 'Build failed because the new rollout failed.'
                            exit 1
                        fi
                    '''
                }
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
