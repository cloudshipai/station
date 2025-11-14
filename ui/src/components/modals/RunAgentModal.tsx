import React, { useState, useEffect } from 'react';
import { X, Play, AlertCircle } from 'lucide-react';
import { apiClient } from '../../api/client';
import type { Agent } from '../../types/station';

interface RunAgentModalProps {
  isOpen: boolean;
  onClose: () => void;
  agent: Agent;
  onSuccess?: (runId: number) => void;
}

interface InputField {
  name: string;
  type: string;
  description?: string;
  required: boolean;
}

export const RunAgentModal: React.FC<RunAgentModalProps> = ({
  isOpen,
  onClose,
  agent,
  onSuccess,
}) => {
  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [inputFields, setInputFields] = useState<InputField[]>([]);
  const [executing, setExecuting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (isOpen && agent.input_schema) {
      try {
        const schema = JSON.parse(agent.input_schema);
        const fields: InputField[] = [];

        if (schema.schema && schema.schema.properties) {
          const properties = schema.schema.properties;
          const required = schema.schema.required || [];

          Object.keys(properties).forEach((key) => {
            const prop = properties[key];
            fields.push({
              name: key,
              type: prop.type || 'string',
              description: prop.description,
              required: required.includes(key),
            });
          });
        }

        setInputFields(fields);
        
        // Initialize inputs with empty values
        const initialInputs: Record<string, string> = {};
        fields.forEach(field => {
          initialInputs[field.name] = '';
        });
        setInputs(initialInputs);
      } catch (err) {
        console.error('Failed to parse input schema:', err);
        setInputFields([]);
      }
    } else {
      // No schema - just provide a simple text input
      setInputFields([{
        name: 'userInput',
        type: 'string',
        description: 'Task description for the agent',
        required: true,
      }]);
      setInputs({ userInput: '' });
    }
    setError('');
  }, [isOpen, agent.input_schema]);

  const handleInputChange = (name: string, value: string) => {
    setInputs(prev => ({
      ...prev,
      [name]: value,
    }));
  };

  const handleRun = async () => {
    // Validate required fields
    for (const field of inputFields) {
      if (field.required && !inputs[field.name]?.trim()) {
        setError(`${field.name} is required`);
        return;
      }
    }

    setExecuting(true);
    setError('');

    try {
      // For now, the API expects a 'task' field, so we'll use userInput or the first field
      const taskValue = inputs.userInput || inputs[inputFields[0]?.name] || Object.values(inputs)[0] || '';
      
      // Execute the agent with the task
      const response = await apiClient.post(`/agents/${agent.id}/execute`, {
        task: taskValue
      });
      
      if (onSuccess && response.data.run_id) {
        onSuccess(response.data.run_id);
      }
      onClose();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to execute agent');
    } finally {
      setExecuting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Play className="h-6 w-6 text-tokyo-green" />
            <div>
              <h2 className="text-xl font-mono font-semibold text-tokyo-fg">
                Run Agent
              </h2>
              <p className="text-sm text-tokyo-comment font-mono">{agent.name}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        {/* Agent Description */}
        {agent.description && (
          <div className="mb-6 p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
            <p className="text-sm text-tokyo-fg font-mono">{agent.description}</p>
          </div>
        )}

        {/* Input Fields */}
        <div className="space-y-4 mb-6">
          {inputFields.map((field) => (
            <div key={field.name}>
              <label className="block text-sm font-mono text-tokyo-fg mb-2">
                {field.name}
                {field.required && <span className="text-tokyo-red ml-1">*</span>}
              </label>
              {field.description && (
                <p className="text-xs text-tokyo-comment font-mono mb-2">
                  {field.description}
                </p>
              )}
              {field.type === 'string' ? (
                <textarea
                  value={inputs[field.name] || ''}
                  onChange={(e) => handleInputChange(field.name, e.target.value)}
                  placeholder={`Enter ${field.name}...`}
                  rows={4}
                  className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono rounded focus:outline-none focus:border-tokyo-blue resize-y"
                />
              ) : (
                <input
                  type="text"
                  value={inputs[field.name] || ''}
                  onChange={(e) => handleInputChange(field.name, e.target.value)}
                  placeholder={`Enter ${field.name}...`}
                  className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono rounded focus:outline-none focus:border-tokyo-blue"
                />
              )}
            </div>
          ))}
        </div>

        {/* Error Message */}
        {error && (
          <div className="mb-4 p-3 bg-tokyo-red/20 border border-tokyo-red rounded flex items-center gap-2">
            <AlertCircle className="h-4 w-4 text-tokyo-red" />
            <p className="text-sm text-tokyo-red font-mono">{error}</p>
          </div>
        )}

        {/* Info about max steps */}
        <div className="mb-4 p-3 bg-tokyo-blue/20 border border-tokyo-blue7 rounded">
          <p className="text-xs text-tokyo-comment font-mono">
            This agent will execute with a maximum of {agent.max_steps} steps
          </p>
        </div>

        {/* Actions */}
        <div className="flex gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono rounded hover:bg-tokyo-dark2 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleRun}
            disabled={executing}
            className="px-4 py-2 bg-tokyo-green text-tokyo-bg font-mono rounded hover:bg-tokyo-green/80 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {executing ? (
              <>
                <span className="animate-spin">⏳</span>
                Executing...
              </>
            ) : (
              <>
                <Play className="h-4 w-4" />
                Run Agent
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};
