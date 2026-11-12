
pipeline {
    agent {
        kubernetes {
            yaml '''
apiVersion: v1
kind: Pod
spec:
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

  volumes:
    - name: buildkit-data
      emptyDir: {}
'''
        }
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

                            IMAGE="ghcr.io/tneftali101/nginx-lab:${GIT_COMMIT}"

                            echo "Publishing image: $IMAGE"

                            buildctl-daemonless.sh build \
                              --frontend dockerfile.v0 \
                              --local context=app \
                              --local dockerfile=app \
                              --output "type=image,name=$IMAGE,push=true"

                            echo "Image published successfully!"
                        '''
                    }
                }
            }
        }

        stage('Test NGINX') {
            steps {
                echo 'Checking deployed NGINX availability...'

                container('curl') {
                    sh '''
                        curl --fail --silent --show-error \
                          http://nginx-service.monitoring-lab.svc.cluster.local/ \
                          --output /dev/null
                    '''
                }

                echo 'NGINX responded successfully!'
            }
        }
    }
}
