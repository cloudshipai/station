# Station + Argo Workflows

Run Station agents in Argo Workflows (Kubernetes-native workflow engine).

## Quick Start

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: station-agent
spec:
  entrypoint: run-agent
  templates:
  - name: run-agent
    container:
      image: ghcr.io/cloudshipai/station:latest
      command: [stn]
      args:
        - agent
        - run
        - "Code Reviewer"
        - "Review the code"
      env:
      - name: OPENAI_API_KEY
        valueFrom:
          secretKeyRef:
            name: station-secrets
            key: openai-api-key
```

## Complete Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: station-pipeline-
spec:
  entrypoint: analysis-pipeline
  arguments:
    parameters:
    - name: repo-url
      value: https://github.com/your-org/your-repo.git

  volumeClaimTemplates:
  - metadata:
      name: workspace
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi

  templates:
  - name: analysis-pipeline
    steps:
    - - name: checkout
        template: git-checkout
    - - name: code-review
        template: run-agent
        arguments:
          parameters:
          - name: agent
            value: "Code Reviewer"
          - name: task
            value: "Review code for bugs and best practices"
      - name: security-scan
        template: run-agent
        arguments:
          parameters:
          - name: agent
            value: "Security Analyst"
          - name: task
            value: "Scan for security vulnerabilities"

  - name: git-checkout
    container:
      image: alpine/git
      command: [sh, -c]
      args:
        - git clone {{workflow.parameters.repo-url}} /workspace
      volumeMounts:
      - name: workspace
        mountPath: /workspace

  - name: run-agent
    inputs:
      parameters:
      - name: agent
      - name: task
    container:
      image: ghcr.io/cloudshipai/station:latest
      command: [stn]
      args:
        - agent
        - run
        - "{{inputs.parameters.agent}}"
        - "{{inputs.parameters.task}}"
      env:
      - name: OPENAI_API_KEY
        valueFrom:
          secretKeyRef:
            name: station-secrets
            key: openai-api-key
      volumeMounts:
      - name: workspace
        mountPath: /workspace
```

## Scheduled Workflows (CronWorkflow)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: CronWorkflow
metadata:
  name: daily-analysis
spec:
  schedule: "0 9 * * *"
  timezone: "America/Los_Angeles"
  workflowSpec:
    entrypoint: run-agent
    templates:
    - name: run-agent
      container:
        image: ghcr.io/cloudshipai/station:latest
        command: [stn]
        args:
          - agent
          - run
          - "Report Generator"
          - "Generate daily summary report"
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

## Setup

1. **Create Secret**:
```bash
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY
```

2. **Submit Workflow**:
```bash
argo submit workflow.yaml
```

3. **Watch Progress**:
```bash
argo watch station-agent
```

## WorkflowTemplate (Reusable)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: station-agent
spec:
  templates:
  - name: run
    inputs:
      parameters:
      - name: agent
      - name: task
    container:
      image: ghcr.io/cloudshipai/station:latest
      command: [stn]
      args:
        - agent
        - run
        - "{{inputs.parameters.agent}}"
        - "{{inputs.parameters.task}}"
      env:
      - name: OPENAI_API_KEY
        valueFrom:
          secretKeyRef:
            name: station-secrets
            key: openai-api-key
```

Use in workflows:

```yaml
- name: analyze
  templateRef:
    name: station-agent
    template: run
  arguments:
    parameters:
    - name: agent
      value: "Code Reviewer"
    - name: task
      value: "Review the code"
```
