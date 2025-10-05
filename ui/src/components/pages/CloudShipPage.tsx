import React from 'react';
import { Cloud, Database, Search, TrendingUp, Shield, Rocket, FileCode, AlertTriangle } from 'lucide-react';

interface PresetInfo {
  name: string;
  app: string;
  appType: string;
  description: string;
  useCases: string[];
  icon: React.ComponentType<any>;
}

const presets: PresetInfo[] = [
  {
    name: 'finops',
    app: 'finops',
    appType: 'inventory',
    description: 'Track current infrastructure resources and costs',
    useCases: ['Resource inventory', 'Cost tracking', 'Infrastructure snapshots'],
    icon: Database,
  },
  {
    name: 'finops-investigations',
    app: 'finops',
    appType: 'investigations',
    description: 'Investigate cost spikes and anomalies',
    useCases: ['Cost spike analysis', 'Budget overrun investigation', 'Unexpected charges'],
    icon: Search,
  },
  {
    name: 'finops-opportunities',
    app: 'finops',
    appType: 'opportunities',
    description: 'Identify cost optimization opportunities',
    useCases: ['Cost savings', 'Resource rightsizing', 'Waste elimination'],
    icon: TrendingUp,
  },
  {
    name: 'finops-projections',
    app: 'finops',
    appType: 'projections',
    description: 'Forecast future costs and resource needs',
    useCases: ['Cost forecasting', 'Capacity planning', 'Burn rate projection'],
    icon: TrendingUp,
  },
  {
    name: 'security-inventory',
    app: 'security',
    appType: 'inventory',
    description: 'Catalog current security vulnerabilities and exposure',
    useCases: ['CVE tracking', 'Vulnerability inventory', 'Security posture snapshots'],
    icon: Shield,
  },
  {
    name: 'security-investigations',
    app: 'security',
    appType: 'investigations',
    description: 'Investigate security incidents and breaches',
    useCases: ['Incident response', 'Breach analysis', 'Attack vector investigation'],
    icon: AlertTriangle,
  },
  {
    name: 'deployments-events',
    app: 'deployments',
    appType: 'events',
    description: 'Track deployment events and changes',
    useCases: ['Deployment tracking', 'Change history', 'Release timeline'],
    icon: Rocket,
  },
  {
    name: 'deployments-investigations',
    app: 'deployments',
    appType: 'investigations',
    description: 'Investigate deployment failures and issues',
    useCases: ['Deployment failures', 'Rollback analysis', 'CI/CD troubleshooting'],
    icon: FileCode,
  },
];

const appTypeInfo = [
  {
    name: 'investigations',
    purpose: 'Root cause analysis',
    question: '"Why did X happen?"',
    examples: ['Cost spikes', 'Security breaches', 'Deployment failures'],
  },
  {
    name: 'opportunities',
    purpose: 'Improvement recommendations',
    question: '"What can we optimize?"',
    examples: ['Cost savings', 'Security hardening', 'Performance gains'],
  },
  {
    name: 'projections',
    purpose: 'Future predictions',
    question: '"What will happen?"',
    examples: ['Cost forecasts', 'Capacity planning', 'Trend analysis'],
  },
  {
    name: 'inventory',
    purpose: 'Current state snapshots',
    question: '"What exists now?"',
    examples: ['Resource lists', 'Vulnerability scans', 'Service catalog'],
  },
  {
    name: 'events',
    purpose: 'Change tracking',
    question: '"What changed?"',
    examples: ['Deployments', 'Incidents', 'Config changes'],
  },
];

export const CloudShipPage: React.FC = () => {
  return (
    <div className="h-full overflow-auto bg-gray-50 dark:bg-gray-950">
      <div className="max-w-7xl mx-auto p-6">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-4">
            <Cloud size={32} className="text-blue-600" />
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white">CloudShip Data Ingestion</h1>
          </div>
          <p className="text-gray-600 dark:text-gray-400">
            Learn how to create agents that send structured intelligence to CloudShip for cross-domain correlation and analysis.
          </p>
        </div>

        {/* Overview */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow p-6 mb-6">
          <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">Overview</h2>
          <p className="text-gray-700 dark:text-gray-300 mb-4">
            CloudShip's agent data standardization enables your agents to send <strong>intelligence findings</strong> not raw data.
            Each finding goes to a standardized subtype table based on <strong>intent</strong>, enabling cross-domain correlation
            while maintaining complete flexibility in data structure.
          </p>
          <div className="bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded p-4">
            <p className="text-sm text-blue-900 dark:text-blue-100">
              <strong>Key Principle:</strong> 5 standard subtypes work across ALL domains (FinOps, Security, Deployments, etc.)
            </p>
          </div>
        </div>

        {/* Standard App Types */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow p-6 mb-6">
          <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">5 Standard App Types</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            These subtypes work across ALL domains and enable CloudShip to correlate findings across agents.
          </p>
          <div className="space-y-4">
            {appTypeInfo.map((appType) => (
              <div key={appType.name} className="border border-gray-200 dark:border-gray-800 rounded-lg p-4">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{appType.name}</h3>
                    <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">{appType.purpose}</p>
                    <p className="text-sm text-blue-600 dark:text-blue-400 mt-1 italic">{appType.question}</p>
                  </div>
                </div>
                <div className="mt-3">
                  <p className="text-xs text-gray-500 dark:text-gray-500 mb-2">Examples:</p>
                  <div className="flex gap-2 flex-wrap">
                    {appType.examples.map((example) => (
                      <span
                        key={example}
                        className="text-xs px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 rounded"
                      >
                        {example}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Preset Schemas */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow p-6 mb-6">
          <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">Available Preset Schemas</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            Station provides preset output schemas to help you quickly create agents with standardized data structures.
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {presets.map((preset) => {
              const Icon = preset.icon;
              return (
                <div key={preset.name} className="border border-gray-200 dark:border-gray-800 rounded-lg p-4">
                  <div className="flex items-start gap-3">
                    <div className="p-2 bg-blue-50 dark:bg-blue-950 rounded">
                      <Icon size={20} className="text-blue-600 dark:text-blue-400" />
                    </div>
                    <div className="flex-1">
                      <h3 className="font-semibold text-gray-900 dark:text-white">{preset.name}</h3>
                      <div className="flex gap-2 mt-1">
                        <span className="text-xs px-2 py-0.5 bg-purple-100 dark:bg-purple-950 text-purple-700 dark:text-purple-300 rounded">
                          {preset.app}
                        </span>
                        <span className="text-xs px-2 py-0.5 bg-green-100 dark:bg-green-950 text-green-700 dark:text-green-300 rounded">
                          {preset.appType}
                        </span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400 mt-2">{preset.description}</p>
                      <div className="mt-2">
                        <p className="text-xs text-gray-500 dark:text-gray-500 mb-1">Use cases:</p>
                        <ul className="text-xs text-gray-600 dark:text-gray-400 space-y-0.5">
                          {preset.useCases.map((useCase) => (
                            <li key={useCase}>• {useCase}</li>
                          ))}
                        </ul>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Agent Configuration */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow p-6 mb-6">
          <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">Agent Configuration</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            Add CloudShip metadata to your agent's frontmatter to enable data ingestion:
          </p>
          <div className="bg-gray-900 rounded p-4 overflow-x-auto">
            <pre className="text-sm text-green-400">
              {`---
metadata:
  name: "AWS Cost Analyzer"
  description: "Analyzes AWS cost spikes"
  app: finops                    # Domain (user-defined)
  app_type: investigations       # MUST be one of 5 standard types
model: gpt-4o-mini
max_steps: 8
tools:
  - "__read_text_file"
  - "__list_directory"
output:
  schema:
    type: object
    properties:
      finding:
        type: string
      evidence:
        type: object
      confidence:
        type: number
---`}
            </pre>
          </div>
        </div>

        {/* Using Presets */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow p-6 mb-6">
          <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">Using Presets</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            When creating or updating an agent via the MCP API, specify an output_schema_preset instead of manually defining the schema:
          </p>
          <div className="bg-gray-50 dark:bg-gray-950 border border-gray-200 dark:border-gray-800 rounded p-4">
            <code className="text-sm text-gray-700 dark:text-gray-300">
              output_schema_preset: "finops-investigations"
            </code>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400 mt-4">
            The preset automatically sets the app and app_type fields and provides a standardized output schema.
          </p>
        </div>

        {/* Validation Rules */}
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow p-6">
          <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">Validation Rules</h2>
          <ul className="space-y-2 text-sm text-gray-700 dark:text-gray-300">
            <li className="flex items-start gap-2">
              <span className="text-blue-600 dark:text-blue-400">•</span>
              <span><strong>app:</strong> Can be anything (user-defined domains allowed - finops, security, deployments, custom, etc.)</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-blue-600 dark:text-blue-400">•</span>
              <span><strong>app_type:</strong> MUST be one of: investigations, opportunities, projections, inventory, events</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-blue-600 dark:text-blue-400">•</span>
              <span><strong>output_schema:</strong> Completely flexible - agent defines structure or uses preset</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-blue-600 dark:text-blue-400">•</span>
              <span><strong>Data ingestion:</strong> Only happens when agent has both app AND app_type plus a schema or preset</span>
            </li>
          </ul>
        </div>
      </div>
    </div>
  );
};
