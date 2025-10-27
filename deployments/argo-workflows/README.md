# Station + Argo Workflows Integration

Run Station security agents in Argo Workflows (Kubernetes-native workflow engine).

## Quick Start

Create a workflow:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: station-security-scan
spec:
  entrypoint: security-scan
  templates:
  - name: security-scan
    container:
      image: ghcr.io/cloudshipai/station-security:latest
      command: [stn]
      args:
        - agent
        - run
        - "Infrastructure Security Auditor"
        - "Scan for security vulnerabilities"
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
  generateName: station-security-pipeline-
  namespace: security
spec:
  entrypoint: security-pipeline
  volumeClaimTemplates:
  - metadata:
      name: workspace
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi

  templates:
  - name: security-pipeline
    steps:
    - - name: checkout
        template: git-checkout
    - - name: infrastructure-scan
        template: infrastructure-security
      - name: supply-chain-scan
        template: supply-chain-security
    - - name: deployment-gate
        template: deployment-gate

  - name: git-checkout
    container:
      image: alpine/git
      command: [sh, -c]
      args:
        - git clone {{workflow.parameters.repo-url}} /workspace
      volumeMounts:
      - name: workspace
        mountPath: /workspace

  - name: infrastructure-security
    container:
      image: ghcr.io/cloudshipai/station-security:latest
      command: [stn]
      args:
        - agent
        - run
        - "Infrastructure Security Auditor"
        - "Scan terraform, kubernetes, and docker for security issues"
      env:
      - name: OPENAI_API_KEY
        valueFrom:
          secretKeyRef:
            name: station-secrets
            key: openai-api-key
      - name: PROJECT_ROOT
        value: /workspace
      volumeMounts:
      - name: workspace
        mountPath: /workspace

  - name: supply-chain-security
    container:
      image: ghcr.io/cloudshipai/station-security:latest
      command: [stn]
      args:
        - agent
        - run
        - "Supply Chain Guardian"
        - "Generate SBOM and scan dependencies"
      env:
      - name: OPENAI_API_KEY
        valueFrom:
          secretKeyRef:
            name: station-secrets
            key: openai-api-key
      volumeMounts:
      - name: workspace
        mountPath: /workspace

  - name: deployment-gate
    container:
      image: ghcr.io/cloudshipai/station-security:latest
      command: [stn]
      args:
        - agent
        - run
        - "Deployment Security Gate"
        - "Validate security posture before deployment"
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

Daily FinOps analysis:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: CronWorkflow
metadata:
  name: daily-cost-analysis
  namespace: finops
spec:
  schedule: "0 9 * * *"  # 9 AM daily
  timezone: "America/Los_Angeles"
  workflowSpec:
    entrypoint: cost-analysis
    templates:
    - name: cost-analysis
      container:
        image: ghcr.io/cloudshipai/station-security:latest
        command: [stn]
        args:
          - agent
          - run
          - "AWS Cost Analyzer"
          - "Analyze AWS costs and identify optimization opportunities"
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: secret-access-key
```

## Setup

1. **Create Secret**:
```bash
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  -n security
```

2. **Submit Workflow**:
```bash
argo submit workflow.yaml
```

3. **Watch Progress**:
```bash
argo watch station-security-scan
```

## WorkflowTemplate (Reusable)

Create reusable template:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: station-security-scan
  namespace: security
spec:
  templates:
  - name: scan
    inputs:
      parameters:
      - name: agent
      - name: task
    container:
      image: ghcr.io/cloudshipai/station-security:latest
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
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: security-scan-
spec:
  entrypoint: main
  templates:
  - name: main
    steps:
    - - name: infrastructure
        templateRef:
          name: station-security-scan
          template: scan
        arguments:
          parameters:
          - name: agent
            value: "Infrastructure Security Auditor"
          - name: task
            value: "Scan for security issues"
```
