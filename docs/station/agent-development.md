# Agent Development

Create agents using the declarative dotprompt format.

## Dotprompt Format

Dotprompt is a simple, declarative format for defining AI agents. Here's a complete example:

```yaml
---
metadata:
  name: "AWS Cost Spike Analyzer"
  description: "Detects unusual cost increases and identifies root causes"
  tags: ["finops", "aws", "cost-analysis"]
model: gpt-4o-mini
max_steps: 5
tools:
  - "__get_cost_and_usage"
  - "__list_cost_allocation_tags"
  - "__get_savings_plans_coverage"
---

{{role "system"}}
You are a FinOps analyst specializing in AWS cost anomaly detection.
Analyze cost trends, identify spikes, and provide actionable recommendations.

{{role "user"}}
{{userInput}}
```

## Development Workflow

### 1. Local Development
[Content to be added]

### 2. Testing with Genkit Developer UI
```bash
stn up --develop
```

### 3. Debugging and Iteration
[Content to be added]

## Best Practices

[Content to be added]
