
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
'''
        }
    }

    stages {
        stage('Build') {
            steps {
                echo 'Build stage triggered from a GitHub update! - Attempt 2'
            }
        }

        stage('Test NGINX') {
            steps {
                echo 'Checking NGINX availability...'

                container('curl') {
                    sh '''
                        id
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
