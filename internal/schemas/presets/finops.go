package presets

// FinOpsSchema defines a picoschema format for GenKit dotprompt compatibility
const FinOpsSchema = `type: object
properties:
  summary:
    type: object
    properties:
      total_estimated_cost:
        type: number
        description: "Total estimated cost in USD"
      cost_optimization_potential:
        type: number
        description: "Potential savings in USD"
      cost_category:
        type: string
        enum: [low, medium, high, critical]
        description: "Overall cost category"
      main_cost_drivers:
        type: array
        items:
          type: string
        description: "Primary cost drivers identified"
    required: [cost_category, main_cost_drivers]
  resource_analysis:
    type: array
    items:
      type: object
      properties:
        resource_type:
          type: string
          description: "Type of resource (e.g., EC2, RDS, S3)"
        resource_name:
          type: string
          description: "Name or identifier of the resource"
        estimated_monthly_cost:
          type: number
          description: "Estimated monthly cost in USD"
        optimization_recommendations:
          type: array
          items:
            type: object
            properties:
              action:
                type: string
                description: "Recommended action"
              potential_savings:
                type: number
                description: "Potential savings in USD"
              impact:
                type: string
                enum: [low, medium, high]
              effort:
                type: string
                enum: [low, medium, high]
            required: [action, impact, effort]
      required: [resource_type, resource_name]
  recommendations:
    type: object
    properties:
      immediate_actions:
        type: array
        items:
          type: string
        description: "Actions that can be taken immediately"
      short_term_optimizations:
        type: array
        items:
          type: string
        description: "Optimizations for next 1-3 months"
      long_term_strategies:
        type: array
        items:
          type: string
        description: "Long-term cost optimization strategies"
required: [summary, resource_analysis, recommendations]`
