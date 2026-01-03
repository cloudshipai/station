# Station + Jenkins

Run Station agents in Jenkins pipelines.

## Quick Start

Add to your `Jenkinsfile`:

```groovy
pipeline {
  agent {
    docker { image 'ghcr.io/cloudshipai/station:latest' }
  }

  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
  }

  stages {
    stage('Analyze') {
      steps {
        sh 'stn agent run "Code Reviewer" "Review the code"'
      }
    }
  }
}
```

## Complete Example

```groovy
pipeline {
  agent {
    docker {
      image 'ghcr.io/cloudshipai/station:latest'
      args '-v /var/run/docker.sock:/var/run/docker.sock'
    }
  }

  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
  }

  stages {
    stage('Code Review') {
      when {
        anyOf {
          branch 'main'
          changeRequest()
        }
      }
      steps {
        sh 'stn agent run "Code Reviewer" "Review code for bugs and best practices"'
      }
    }

    stage('Security Scan') {
      steps {
        sh 'stn agent run "Security Analyst" "Scan for security vulnerabilities"'
      }
    }
  }

  post {
    always {
      echo 'Analysis completed'
    }
  }
}
```

## Using Different AI Providers

### Anthropic Claude

```groovy
environment {
  ANTHROPIC_API_KEY = credentials('anthropic-api-key')
  STN_AI_PROVIDER = 'anthropic'
  STN_AI_MODEL = 'claude-3-5-sonnet-20241022'
}
```

### Google Gemini

```groovy
environment {
  GOOGLE_API_KEY = credentials('google-api-key')
  STN_AI_PROVIDER = 'gemini'
  STN_AI_MODEL = 'gemini-2.0-flash-exp'
}
```

## Scheduled Jobs

```groovy
pipeline {
  agent {
    docker { image 'ghcr.io/cloudshipai/station:latest' }
  }

  triggers {
    cron('0 9 * * *')
  }

  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
  }

  stages {
    stage('Daily Analysis') {
      steps {
        sh 'stn agent run "Report Generator" "Generate daily summary"'
      }
    }
  }
}
```

## Setup

1. **Add Credentials** (Manage Jenkins > Credentials):
   - ID: `openai-api-key`
   - Type: Secret text
   - Value: Your OpenAI API key

2. **Create your agents** in `environments/default/template.json`

3. **Create Pipeline** job pointing to your repo

4. **Run** - pipeline executes automatically
