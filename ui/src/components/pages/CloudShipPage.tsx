import React from 'react';
import { DollarSign, Database, Search, TrendingUp, TrendingDown, Calendar, Lightbulb, Target, BarChart } from 'lucide-react';

interface PresetInfo {
  name: string;
  app: string;
  appType: string;
  description: string;
  whatItDoes: string;
  exampleQuestions: string[];
  typicalData: string[];
  icon: React.ComponentType<any>;
}

const finopsPresets: PresetInfo[] = [
  {
    name: 'finops-inventory',
    app: 'finops',
    appType: 'inventory',
    description: 'Track current infrastructure resources and their costs across cloud providers',
    whatItDoes: 'Creates snapshots of all cloud resources with current cost allocation, tags, and metadata',
    exampleQuestions: [
      'What are all my AWS resources and their monthly costs?',
      'Show me all EC2 instances tagged with "production"',
      'List all resources in us-east-1 with their costs'
    ],
    typicalData: ['resource_id', 'resource_type', 'cost', 'region', 'tags', 'metadata'],
    icon: Database,
  },
  {
    name: 'finops-investigations',
    app: 'finops',
    appType: 'investigations',
    description: 'Investigate cost spikes, anomalies, and unexpected charges',
    whatItDoes: 'Analyzes cost data to identify root causes of cost increases, budget overruns, or billing surprises',
    exampleQuestions: [
      'Why did my AWS bill spike 300% last week?',
      'What caused the unexpected $5k charge on Tuesday?',
      'Which service is driving our cost increase?'
    ],
    typicalData: ['finding', 'root_cause', 'cost_impact', 'time_range', 'affected_resources', 'evidence'],
    icon: Search,
  },
  {
    name: 'finops-opportunities',
    app: 'finops',
    appType: 'opportunities',
    description: 'Identify cost optimization opportunities and savings recommendations',
    whatItDoes: 'Finds waste, overprovisioned resources, and cost-saving opportunities with actionable recommendations',
    exampleQuestions: [
      'What cost optimizations can I make right now?',
      'Which resources are underutilized and costing money?',
      'Show me all savings opportunities over $100/month'
    ],
    typicalData: ['opportunity_type', 'potential_savings', 'affected_resources', 'recommendation', 'effort', 'risk'],
    icon: TrendingDown,
  },
  {
    name: 'finops-projections',
    app: 'finops',
    appType: 'projections',
    description: 'Forecast future costs, burn rates, and capacity needs',
    whatItDoes: 'Predicts future costs based on historical trends, planned changes, and growth patterns',
    exampleQuestions: [
      'What will my AWS costs be next quarter?',
      'When will we hit our $50k monthly budget?',
      'Project costs if we double our infrastructure'
    ],
    typicalData: ['projection_period', 'forecasted_cost', 'confidence_interval', 'assumptions', 'trend_data'],
    icon: TrendingUp,
  },
  {
    name: 'finops-events',
    app: 'finops',
    appType: 'events',
    description: 'Track cost-related events like resource changes, budget alerts, and billing milestones',
    whatItDoes: 'Captures significant cost events and changes that impact your cloud spend',
    exampleQuestions: [
      'What cost events happened this week?',
      'Show me all resources created in the last 24 hours',
      'When did we cross the $10k monthly threshold?'
    ],
    typicalData: ['event_type', 'timestamp', 'cost_impact', 'resource', 'change_description'],
    icon: Calendar,
  },
];

export const CloudShipPage: React.FC = () => {
  return (
    <div className="h-full overflow-auto bg-tokyo-bg">
      <div className="max-w-7xl mx-auto p-6">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-4">
            <DollarSign size={36} className="text-tokyo-green" />
            <h1 className="text-3xl font-bold font-mono text-tokyo-blue">FinOps Agents for CloudShip</h1>
          </div>
          <p className="text-tokyo-comment text-lg">
            Learn how to create FinOps agents that send cost intelligence findings to CloudShip/Lighthouse for analysis and correlation.
          </p>
        </div>

        {/* What is FinOps Intelligence? */}
        <div className="bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-6 mb-6">
          <div className="flex items-center gap-2 mb-4">
            <Lightbulb className="h-6 w-6 text-tokyo-yellow" />
            <h2 className="text-xl font-bold font-mono text-tokyo-blue">What is FinOps Intelligence?</h2>
          </div>
          <p className="text-tokyo-fg mb-4">
            FinOps agents send <strong className="text-tokyo-orange">findings</strong>, not raw data. Each agent answers a specific question about your cloud costs and sends structured intelligence to CloudShip.
          </p>
          <div className="bg-tokyo-bg border-l-4 border-tokyo-green rounded p-4">
            <p className="text-sm text-tokyo-green font-mono">
              <strong>Key Principle:</strong> Agents provide intelligence (insights, recommendations, root causes) rather than dumping raw billing data.
            </p>
          </div>
        </div>

        {/* 5 Types of FinOps Agents */}
        <div className="bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-6 mb-6">
          <div className="flex items-center gap-2 mb-4">
            <Target className="h-6 w-6 text-tokyo-purple" />
            <h2 className="text-xl font-bold font-mono text-tokyo-blue">5 Types of FinOps Agents</h2>
          </div>
          <p className="text-tokyo-comment mb-6 text-sm">
            All FinOps agents follow one of these 5 standard patterns. This enables CloudShip to correlate findings across different agents and domains.
          </p>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {finopsPresets.map((preset) => {
              const Icon = preset.icon;
              return (
                <div key={preset.name} className="bg-tokyo-bg border border-tokyo-dark3 rounded-lg p-5 hover:border-tokyo-blue transition-colors">
                  <div className="flex items-start gap-3 mb-4">
                    <div className="p-2 bg-tokyo-dark1 rounded">
                      <Icon size={24} className="text-tokyo-blue" />
                    </div>
                    <div className="flex-1">
                      <h3 className="font-semibold font-mono text-tokyo-cyan text-lg">{preset.name}</h3>
                      <div className="flex gap-2 mt-2">
                        <span className="text-xs px-2 py-0.5 border border-tokyo-purple text-tokyo-purple rounded font-mono">
                          {preset.appType}
                        </span>
                      </div>
                    </div>
                  </div>

                  <p className="text-tokyo-fg text-sm mb-4">{preset.description}</p>

                  <div className="mb-4">
                    <div className="text-xs text-tokyo-comment font-mono mb-2">WHAT IT DOES:</div>
                    <p className="text-sm text-tokyo-comment italic">{preset.whatItDoes}</p>
                  </div>

                  <div className="mb-4">
                    <div className="text-xs text-tokyo-comment font-mono mb-2">EXAMPLE QUESTIONS:</div>
                    <ul className="text-sm text-tokyo-green space-y-1">
                      {preset.exampleQuestions.map((q, i) => (
                        <li key={i} className="flex items-start gap-2">
                          <span className="text-tokyo-green">•</span>
                          <span className="italic">"{q}"</span>
                        </li>
                      ))}
                    </ul>
                  </div>

                  <div>
                    <div className="text-xs text-tokyo-comment font-mono mb-2">TYPICAL DATA FIELDS:</div>
                    <div className="flex gap-2 flex-wrap">
                      {preset.typicalData.map((field) => (
                        <span
                          key={field}
                          className="text-xs px-2 py-1 bg-tokyo-dark1 text-tokyo-orange rounded font-mono"
                        >
                          {field}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* How to Create FinOps Agents */}
        <div className="bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-6 mb-6">
          <div className="flex items-center gap-2 mb-4">
            <BarChart className="h-6 w-6 text-tokyo-cyan" />
            <h2 className="text-xl font-bold font-mono text-tokyo-blue">Creating FinOps Agents</h2>
          </div>

          <div className="space-y-6">
            {/* Step 1 */}
            <div>
              <div className="text-lg font-mono text-tokyo-green mb-2">Step 1: Choose Agent Type</div>
              <p className="text-tokyo-comment mb-3">
                Pick one of the 5 agent types based on what question you want to answer. Each type has a standard output schema preset.
              </p>
              <div className="bg-tokyo-bg rounded p-3 border border-tokyo-dark3">
                <div className="text-xs text-tokyo-comment font-mono mb-2">Example:</div>
                <p className="text-sm text-tokyo-fg">
                  "I want to investigate why costs spiked" → Use <span className="text-tokyo-orange font-mono">finops-investigations</span>
                </p>
              </div>
            </div>

            {/* Step 2 */}
            <div>
              <div className="text-lg font-mono text-tokyo-green mb-2">Step 2: Use MCP to Create Agent</div>
              <p className="text-tokyo-comment mb-3">
                Create your agent via MCP tools and specify the output_schema_preset:
              </p>
              <div className="bg-tokyo-dark2 rounded p-4 overflow-x-auto border border-tokyo-dark3">
                <pre className="text-sm text-tokyo-green font-mono">
{`create_agent({
  name: "AWS Cost Spike Investigator",
  description: "Analyzes AWS cost spikes and identifies root causes",
  prompt: "You are a FinOps cost investigator...",
  environment_id: "default",
  output_schema_preset: "finops-investigations"  # Auto-sets schema + app metadata
})`}
                </pre>
              </div>
            </div>

            {/* Step 3 */}
            <div>
              <div className="text-lg font-mono text-tokyo-green mb-2">Step 3: Agent Runs & Data Ingestion</div>
              <p className="text-tokyo-comment mb-3">
                When your agent runs, its output is validated against the schema and sent to CloudShip/Lighthouse:
              </p>
              <ul className="space-y-2 text-sm text-tokyo-fg">
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-blue">→</span>
                  <span>Agent executes and returns structured JSON matching the schema</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-blue">→</span>
                  <span>Station validates output against the finops-investigations schema</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-blue">→</span>
                  <span>Validated finding is sent to CloudShip with app="finops" and app_type="investigations"</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-blue">→</span>
                  <span>CloudShip stores it in the investigations table for correlation with other findings</span>
                </li>
              </ul>
            </div>
          </div>
        </div>

        {/* Schema Builder */}
        <div className="bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-6">
          <h2 className="text-xl font-bold font-mono text-tokyo-blue mb-4">Using the Schema Builder</h2>
          <p className="text-tokyo-comment mb-4">
            When editing an agent, the right-hand panel shows a visual schema builder where you can:
          </p>
          <ul className="space-y-2 text-sm text-tokyo-fg mb-4">
            <li className="flex items-start gap-2">
              <span className="text-tokyo-green">✓</span>
              <span>View the preset schema structure in 3 modes: Form Builder, Visual Preview, or Code Editor</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-tokyo-green">✓</span>
              <span>Add custom fields to the preset schema (e.g., add "vendor" or "team" fields)</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-tokyo-green">✓</span>
              <span>Build nested objects and arrays with expand/collapse controls</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-tokyo-green">✓</span>
              <span>Toggle required fields and add descriptions</span>
            </li>
          </ul>
          <div className="bg-tokyo-bg border-l-4 border-tokyo-cyan rounded p-4">
            <p className="text-sm text-tokyo-cyan font-mono">
              💡 Tip: Start with a preset schema, then customize it for your specific needs. The YAML editor and schema builder stay in sync!
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};
