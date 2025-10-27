# Station + Tekton Integration

Run Station security agents in Tekton Pipelines (Kubernetes-native CI/CD).

## Quick Start

Create a Task:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: station-security-scan
spec:
  params:
  - name: agent
    type: string
  - name: task
    type: string
  steps:
  - name: scan
    image: ghcr.io/cloudshipai/station-security:latest
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
  name: station-security-pipeline
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

  - name: infrastructure-security
    runAfter: [git-clone]
    taskRef:
      name: station-security-scan
    workspaces:
    - name: source
      workspace: source
    params:
    - name: agent
      value: "Infrastructure Security Auditor"
    - name: task
      value: "Scan terraform, kubernetes, and docker for security vulnerabilities"

  - name: supply-chain-security
    runAfter: [git-clone]
    taskRef:
      name: station-security-scan
    workspaces:
    - name: source
      workspace: source
    params:
    - name: agent
      value: "Supply Chain Guardian"
    - name: task
      value: "Generate SBOM and scan dependencies"

  - name: deployment-gate
    runAfter: [infrastructure-security, supply-chain-security]
    taskRef:
      name: station-security-scan
    workspaces:
    - name: source
      workspace: source
    params:
    - name: agent
      value: "Deployment Security Gate"
    - name: task
      value: "Validate security posture before deployment"
```

## Task Definition

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: station-security-scan
  namespace: security
spec:
  params:
  - name: agent
    type: string
    description: Station agent to run
  - name: task
    type: string
    description: Task description for the agent
  workspaces:
  - name: source
    description: Workspace containing source code
  steps:
  - name: security-scan
    image: ghcr.io/cloudshipai/station-security:latest
    workingDir: $(workspaces.source.path)
    script: |
      #!/bin/sh
      set -e
      echo "Running Station agent: $(params.agent)"
      stn agent run "$(params.agent)" "$(params.task)"
    env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: station-secrets
          key: openai-api-key
    - name: PROJECT_ROOT
      value: $(workspaces.source.path)
```

## Trigger with PipelineRun

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: station-security-
  namespace: security
spec:
  pipelineRef:
    name: station-security-pipeline
  params:
  - name: repo-url
    value: https://github.com/your-org/your-repo.git
  workspaces:
  - name: source
    volumeClaimTemplate:
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```

## Scheduled Scans (CronJob + Tekton)

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-cost-analysis
  namespace: finops
spec:
  schedule: "0 9 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: tekton-triggers-sa
          containers:
          - name: trigger
            image: curlimages/curl
            command:
            - sh
            - -c
            - |
              curl -X POST \
                -H "Content-Type: application/json" \
                -d '{"params":[{"name":"agent","value":"AWS Cost Analyzer"}]}' \
                http://el-station-listener.finops.svc.cluster.local:8080
          restartPolicy: OnFailure
```

## Setup

1. **Create Secret**:
```bash
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  -n security
```

2. **Install Task**:
```bash
kubectl apply -f task.yaml
```

3. **Install Pipeline**:
```bash
kubectl apply -f pipeline.yaml
```

4. **Run Pipeline**:
```bash
kubectl create -f pipelinerun.yaml
```

5. **Watch Progress**:
```bash
tkn pipelinerun logs -f -n security
```

## Multi-Agent Task

Run multiple agents in sequence:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: station-multi-agent-scan
spec:
  workspaces:
  - name: source
  steps:
  - name: infrastructure
    image: ghcr.io/cloudshipai/station-security:latest
    workingDir: $(workspaces.source.path)
    script: |
      stn agent run "Infrastructure Security Auditor" "Scan infrastructure"
    env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: station-secrets
          key: openai-api-key

  - name: supply-chain
    image: ghcr.io/cloudshipai/station-security:latest
    workingDir: $(workspaces.source.path)
    script: |
      stn agent run "Supply Chain Guardian" "Scan dependencies"
    env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: station-secrets
          key: openai-api-key

  - name: deployment-gate
    image: ghcr.io/cloudshipai/station-security:latest
    workingDir: $(workspaces.source.path)
    script: |
      stn agent run "Deployment Security Gate" "Validate deployment"
    env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: station-secrets
          key: openai-api-key
```
