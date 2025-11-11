import React, { useState, useEffect } from 'react';
import { X, Wand2, AlertCircle, CheckCircle, Loader } from 'lucide-react';
import { apiClient } from '../../api/client';

interface FakerBuilderModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentName: string;
  environmentId: number;
  onSuccess?: (fakerName: string, environmentName: string) => void;
}

interface FakerTemplate {
  id: string;
  name: string;
  description: string;
  instruction: string;
  model: string;
  category: string;
  toolsGenerated: string;
}

const FAKER_TEMPLATES: FakerTemplate[] = [
  {
    id: 'aws-finops',
    name: 'AWS FinOps',
    description: 'Complete AWS cost management and optimization tools',
    instruction: 'Generate comprehensive AWS Cost Explorer and Billing API tools for FinOps investigations...',
    model: 'gpt-4o-mini',
    category: 'Cloud Cost Management',
    toolsGenerated: '~25 tools',
  },
  {
    id: 'gcp-finops',
    name: 'GCP FinOps',
    description: 'GCP cloud billing and cost optimization tools',
    instruction: 'Generate comprehensive GCP Cloud Billing and Cost Management API tools...',
    model: 'gpt-4o-mini',
    category: 'Cloud Cost Management',
    toolsGenerated: '~22 tools',
  },
  {
    id: 'azure-finops',
    name: 'Azure FinOps',
    description: 'Azure cost management and optimization tools',
    instruction: 'Generate comprehensive Azure Cost Management API tools...',
    model: 'gpt-4o-mini',
    category: 'Cloud Cost Management',
    toolsGenerated: '~20 tools',
  },
  {
    id: 'datadog-monitoring',
    name: 'Datadog Monitoring',
    description: 'Datadog metrics, logs, and monitoring tools',
    instruction: 'Generate Datadog monitoring API tools for DevOps and observability...',
    model: 'gpt-4o-mini',
    category: 'Observability',
    toolsGenerated: '~18 tools',
  },
  {
    id: 'stripe-payments',
    name: 'Stripe Payments',
    description: 'Stripe payment and subscription API tools',
    instruction: 'Generate Stripe payment API tools for payment processing...',
    model: 'gpt-4o-mini',
    category: 'Payments',
    toolsGenerated: '~15 tools',
  },
];

export const FakerBuilderModal: React.FC<FakerBuilderModalProps> = ({
  isOpen,
  onClose,
  environmentName,
  environmentId,
  onSuccess
}) => {
  const [activeTab, setActiveTab] = useState<'template' | 'custom'>('template');
  const [fakerName, setFakerName] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState<string>('');
  const [customInstruction, setCustomInstruction] = useState('');
  const [aiModel, setAiModel] = useState('gpt-4o-mini');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (!isOpen) {
      // Reset state when modal closes
      setFakerName('');
      setSelectedTemplate('');
      setCustomInstruction('');
      setAiModel('gpt-4o-mini');
      setError(null);
      setSuccess(false);
      setActiveTab('template');
    }
  }, [isOpen]);

  const validateFakerName = (name: string): string | null => {
    if (!name) return 'Faker name is required';
    if (!/^[a-z0-9-]+$/.test(name)) {
      return 'Name can only contain lowercase letters, numbers, and hyphens';
    }
    if (name.length < 3) return 'Name must be at least 3 characters';
    if (name.length > 50) return 'Name must be less than 50 characters';
    return null;
  };

  const handleSubmit = async () => {
    setError(null);

    // Validation
    const nameError = validateFakerName(fakerName);
    if (nameError) {
      setError(nameError);
      return;
    }

    if (activeTab === 'template' && !selectedTemplate) {
      setError('Please select a template');
      return;
    }

    if (activeTab === 'custom' && customInstruction.length < 50) {
      setError('Custom instruction must be at least 50 characters');
      return;
    }

    setIsLoading(true);

    try {
      const payload: any = {
        name: fakerName,
        model: aiModel,
      };

      if (activeTab === 'template') {
        payload.template = selectedTemplate;
      } else {
        payload.instruction = customInstruction;
      }

      const response = await apiClient.post(
        `/environments/${environmentId}/fakers`,
        payload
      );

      if (response.data) {
        setSuccess(true);
        
        // Automatically trigger sync after 1 second
        setTimeout(() => {
          if (onSuccess) {
            onSuccess(fakerName, environmentName);
          }
          onClose();
        }, 1500);
      }
    } catch (err: any) {
      console.error('Failed to create faker:', err);
      const errorMessage = err.response?.data?.error || err.message || 'Failed to create faker';
      setError(errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  if (!isOpen) return null;

  if (success) {
    return (
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
        <div className="bg-gray-900 border-2 border-green-500 rounded-lg p-8 max-w-md w-full mx-4">
          <div className="text-center">
            <CheckCircle className="h-16 w-16 text-green-500 mx-auto mb-4" />
            <h2 className="text-2xl font-mono font-bold text-green-500 mb-2">
              Faker Created!
            </h2>
            <p className="text-gray-400 font-mono mb-4">
              <span className="text-cyan-400">{fakerName}</span> has been created successfully.
            </p>
            <p className="text-gray-400 font-mono text-sm">
              Syncing environment automatically...
            </p>
            <Loader className="h-6 w-6 text-blue-500 mx-auto mt-4 animate-spin" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-gray-900 border-2 border-gray-700 rounded-lg w-full max-w-4xl mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-700">
          <div className="flex items-center gap-3">
            <Wand2 className="h-6 w-6 text-purple-400" />
            <h2 className="text-xl font-mono font-semibold text-gray-100">
              Create Faker
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-800 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-400 hover:text-gray-100" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6">
          {/* Environment Info */}
          <div className="mb-6 p-4 bg-gray-800 border border-gray-700 rounded-lg">
            <div className="text-sm text-gray-400 mb-1">Target Environment:</div>
            <div className="text-lg font-mono text-cyan-400">{environmentName}</div>
          </div>

          {/* Faker Name Input */}
          <div className="mb-6">
            <label className="block text-sm font-mono text-gray-100 mb-2">
              Faker Name <span className="text-red-400">*</span>
            </label>
            <input
              type="text"
              value={fakerName}
              onChange={(e) => setFakerName(e.target.value.toLowerCase())}
              placeholder="e.g., my-aws-costs, gcp-billing-faker"
              className="w-full px-4 py-2 bg-gray-800 border border-gray-700 text-gray-100 font-mono rounded-lg focus:outline-none focus:border-blue-500"
            />
            <div className="text-xs text-gray-400 mt-1">
              Lowercase letters, numbers, and hyphens only
            </div>
          </div>

          {/* Tab Selector */}
          <div className="flex bg-gray-800 rounded-lg p-1 mb-6">
            <button
              onClick={() => setActiveTab('template')}
              className={`flex-1 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                activeTab === 'template'
                  ? 'bg-purple-600 text-white'
                  : 'text-gray-400 hover:text-purple-400'
              }`}
            >
              Use Template
            </button>
            <button
              onClick={() => setActiveTab('custom')}
              className={`flex-1 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                activeTab === 'custom'
                  ? 'bg-purple-600 text-white'
                  : 'text-gray-400 hover:text-purple-400'
              }`}
            >
              Custom Instruction
            </button>
          </div>

          {/* Template Selection */}
          {activeTab === 'template' && (
            <div className="mb-6">
              <label className="block text-sm font-mono text-gray-100 mb-3">
                Select Template <span className="text-red-400">*</span>
              </label>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {FAKER_TEMPLATES.map((template) => (
                  <div
                    key={template.id}
                    onClick={() => setSelectedTemplate(template.id)}
                    className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
                      selectedTemplate === template.id
                        ? 'border-purple-500 bg-purple-900 bg-opacity-20'
                        : 'border-gray-700 hover:border-gray-600 bg-gray-800'
                    }`}
                  >
                    <h3 className="font-mono font-semibold text-cyan-400 mb-1">
                      {template.name}
                    </h3>
                    <p className="text-sm text-gray-400 mb-2">
                      {template.description}
                    </p>
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-purple-400">{template.category}</span>
                      <span className="text-green-400">{template.toolsGenerated}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Custom Instruction */}
          {activeTab === 'custom' && (
            <div className="mb-6">
              <label className="block text-sm font-mono text-gray-100 mb-2">
                AI Instruction <span className="text-red-400">*</span>
              </label>
              <textarea
                value={customInstruction}
                onChange={(e) => setCustomInstruction(e.target.value)}
                placeholder="Describe the tools you want the faker to generate. Be specific about the API operations, parameters, and response formats needed..."
                rows={8}
                className="w-full px-4 py-3 bg-gray-800 border border-gray-700 text-gray-100 font-mono text-sm rounded-lg focus:outline-none focus:border-blue-500 resize-none"
              />
              <div className="text-xs text-gray-400 mt-1">
                {customInstruction.length}/50 characters minimum
              </div>
            </div>
          )}

          {/* AI Model Selection */}
          <div className="mb-6">
            <label className="block text-sm font-mono text-gray-100 mb-2">
              AI Model (Optional)
            </label>
            <select
              value={aiModel}
              onChange={(e) => setAiModel(e.target.value)}
              className="w-full px-4 py-2 bg-gray-800 border border-gray-700 text-gray-100 font-mono rounded-lg focus:outline-none focus:border-blue-500"
            >
              <option value="gpt-4o-mini">gpt-4o-mini (Default)</option>
              <option value="gpt-4o">gpt-4o</option>
              <option value="claude-3-5-sonnet-20241022">claude-3-5-sonnet</option>
            </select>
          </div>

          {/* Error Display */}
          {error && (
            <div className="mb-6 p-4 bg-red-900 bg-opacity-20 border border-red-500 rounded-lg flex items-start gap-3">
              <AlertCircle className="h-5 w-5 text-red-500 flex-shrink-0 mt-0.5" />
              <div className="text-sm text-red-400 font-mono">{error}</div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-6 border-t border-gray-700">
          <button
            onClick={onClose}
            disabled={isLoading}
            className="px-6 py-2 text-gray-400 border border-gray-700 rounded-lg hover:bg-gray-800 font-mono text-sm transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={isLoading || !fakerName}
            className="flex items-center gap-2 px-6 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 font-mono text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLoading ? (
              <>
                <Loader className="h-4 w-4 animate-spin" />
                Creating...
              </>
            ) : (
              <>
                <Wand2 className="h-4 w-4" />
                Create Faker
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};
