# Station + Jenkins Integration

Run Station security agents in Jenkins pipelines.

## Quick Start

Add to your `Jenkinsfile`:

```groovy
pipeline {
  agent {
    docker {
      image 'ghcr.io/cloudshipai/station-security:latest'
    }
  }

  stages {
    stage('Security Scan') {
      steps {
        sh 'stn agent run "Infrastructure Security Auditor" "Scan for security issues"'
      }
    }
  }

  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
  }
}
```

## Complete Example

```groovy
pipeline {
  agent {
    docker {
      image 'ghcr.io/cloudshipai/station-security:latest'
      args '-v /var/run/docker.sock:/var/run/docker.sock --privileged'
    }
  }

  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
    PROJECT_ROOT = "${WORKSPACE}"
  }

  stages {
    stage('Infrastructure Security') {
      when {
        anyOf {
          branch 'main'
          changeRequest()
        }
      }
      steps {
        sh '''
          stn agent run "Infrastructure Security Auditor" \
            "Scan terraform, kubernetes, and docker for security vulnerabilities"
        '''
      }
    }

    stage('Supply Chain') {
      steps {
        sh '''
          stn agent run "Supply Chain Guardian" \
            "Generate SBOM and scan dependencies"
        '''
      }
    }

    stage('Deployment Gate') {
      when {
        branch 'main'
      }
      steps {
        input message: 'Approve deployment?'
        sh '''
          stn agent run "Deployment Security Gate" \
            "Validate security posture before deployment"
        '''
      }
    }
  }

  post {
    always {
      echo 'Security scan completed'
    }
  }
}
```

## Scheduled Scans

Create a Jenkins job with cron trigger:

```groovy
pipeline {
  agent {
    docker { image 'ghcr.io/cloudshipai/station-security:latest' }
  }

  triggers {
    cron('0 9 * * *')  // Daily at 9 AM
  }

  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
    AWS_ACCESS_KEY_ID = credentials('aws-access-key-id')
    AWS_SECRET_ACCESS_KEY = credentials('aws-secret-key')
  }

  stages {
    stage('Daily Cost Analysis') {
      steps {
        sh 'stn agent run "AWS Cost Analyzer" "Analyze AWS costs"'
      }
    }
  }
}
```

## Setup

1. **Add Credentials** (Manage Jenkins → Credentials):
   - ID: `openai-api-key`
   - Secret text: Your OpenAI API key

2. **Copy Jenkinsfile** to your repository

3. **Create Pipeline** job pointing to your repo

4. **Run** - Pipeline executes automatically

## Shared Library (Advanced)

Create `vars/stationScan.groovy`:

```groovy
def call(Map config = [:]) {
  docker.image('ghcr.io/cloudshipai/station-security:latest').inside() {
    sh """
      stn agent run '${config.agent}' '${config.task}'
    """
  }
}
```

Use in Jenkinsfile:

```groovy
@Library('shared-library') _

pipeline {
  agent any
  stages {
    stage('Security') {
      steps {
        stationScan(
          agent: 'Infrastructure Security Auditor',
          task: 'Scan for vulnerabilities'
        )
      }
    }
  }
}
```
