
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

        stage('Build Image') {
            steps {
                echo 'Building our custom NGINX image...'

                container('buildkit') {
                    sh '''
                        buildctl-daemonless.sh build \
                          --frontend dockerfile.v0 \
                          --local context=app \
                          --local dockerfile=app \
                          --output type=oci,dest=/tmp/nginx-lab.oci.tar

                        ls -lh /tmp/nginx-lab.oci.tar
                    '''
                }

                echo 'Container image built successfully!'
            }
        }

        stage('Test NGINX') {
            steps {
                echo 'Checking NGINX availability...'

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
