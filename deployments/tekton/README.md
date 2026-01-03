# Station + Tekton

Run Station agents in Tekton Pipelines (Kubernetes-native CI/CD).

## Quick Start

Create a Task:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: station-agent
spec:
  params:
  - name: agent
    type: string
  - name: task
    type: string
  steps:
  - name: run
    image: ghcr.io/cloudshipai/station:latest
    script: |
      stn agent run "$(params.agent)" "$(params.task)"
    env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: station-secrets
          key: openai-api-key
```

## Complete Pipeline

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: station-pipeline
spec:
  workspaces:
  - name: source
  params:
  - name: repo-url
    type: string

  tasks:
  - name: git-clone
    taskRef:
      name: git-clone
    workspaces:
    - name: output
      workspace: source
    params:
    - name: url
      value: $(params.repo-url)

  - name: code-review
    runAfter: [git-clone]
    taskRef:
      name: station-agent
    workspaces:
    - name: source
      workspace: source
    params:
    - name: agent
      value: "Code Reviewer"
    - name: task
      value: "Review code for bugs and best practices"

  - name: security-scan
    runAfter: [git-clone]
    taskRef:
      name: station-agent
    workspaces:
    - name: source
      workspace: source
    params:
    - name: agent
      value: "Security Analyst"
    - name: task
      value: "Scan for security vulnerabilities"
```

## Task Definition

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: station-agent
spec:
  params:
  - name: agent
    type: string
    description: Agent name to run
  - name: task
    type: string
    description: Task description
  workspaces:
  - name: source
    description: Source code workspace
  steps:
  - name: run-agent
    image: ghcr.io/cloudshipai/station:latest
    workingDir: $(workspaces.source.path)
    script: |
      #!/bin/sh
      set -e
      echo "Running agent: $(params.agent)"
      stn agent run "$(params.agent)" "$(params.task)"
    env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: station-secrets
          key: openai-api-key
```

## Using Different AI Providers

### Anthropic Claude

```yaml
env:
- name: ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: anthropic-api-key
- name: STN_AI_PROVIDER
  value: anthropic
- name: STN_AI_MODEL
  value: claude-3-5-sonnet-20241022
```

### Google Gemini

```yaml
env:
- name: GOOGLE_API_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: google-api-key
- name: STN_AI_PROVIDER
  value: gemini
- name: STN_AI_MODEL
  value: gemini-2.0-flash-exp
```

## Run Pipeline

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: station-run-
spec:
  pipelineRef:
    name: station-pipeline
  params:
  - name: repo-url
    value: https://github.com/your-org/your-repo.git
  workspaces:
  - name: source
    volumeClaimTemplate:
      spec:
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 1Gi
```

## Setup

1. **Create Secret**:
```bash
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY
```

2. **Install Task**:
```bash
kubectl apply -f task.yaml
```

3. **Install Pipeline**:
```bash
kubectl apply -f pipeline.yaml
```

4. **Run**:
```bash
kubectl create -f pipelinerun.yaml
```

5. **Watch**:
```bash
tkn pipelinerun logs -f
```
