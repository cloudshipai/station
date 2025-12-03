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
    <div 
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 animate-in fade-in duration-200"
      onClick={onClose}
    >
      <div 
        className="bg-white border border-gray-200 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto shadow-lg animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Play className="h-6 w-6 text-green-600" />
            <div>
              <h2 className="text-xl font-semibold text-gray-900">
                Run Agent
              </h2>
              <p className="text-sm text-gray-600">{agent.name}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-500 hover:text-gray-900" />
          </button>
        </div>

        {/* Agent Description */}
        {agent.description && (
          <div className="mb-6 p-4 bg-gray-50 border border-gray-200 rounded">
            <p className="text-sm text-gray-700">{agent.description}</p>
          </div>
        )}

        {/* Input Fields */}
        <div className="space-y-4 mb-6">
          {inputFields.map((field) => (
            <div key={field.name}>
              <label className="block text-sm text-gray-700 mb-2 font-medium">
                {field.name}
                {field.required && <span className="text-red-500 ml-1">*</span>}
              </label>
              {field.description && (
                <p className="text-xs text-gray-600 mb-2">
                  {field.description}
                </p>
              )}
              {field.type === 'string' ? (
                <textarea
                  value={inputs[field.name] || ''}
                  onChange={(e) => handleInputChange(field.name, e.target.value)}
                  placeholder={`Enter ${field.name}...`}
                  rows={4}
                  className="w-full px-3 py-2 bg-white border border-gray-300 text-gray-900 rounded focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue resize-y"
                />
              ) : (
                <input
                  type="text"
                  value={inputs[field.name] || ''}
                  onChange={(e) => handleInputChange(field.name, e.target.value)}
                  placeholder={`Enter ${field.name}...`}
                  className="w-full px-3 py-2 bg-white border border-gray-300 text-gray-900 rounded focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue"
                />
              )}
            </div>
          ))}
        </div>

        {/* Error Message */}
        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded flex items-center gap-2">
            <AlertCircle className="h-4 w-4 text-red-600" />
            <p className="text-sm text-red-600">{error}</p>
          </div>
        )}

        {/* Info about max steps */}
        <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded">
          <p className="text-xs text-gray-600">
            This agent will execute with a maximum of {agent.max_steps} steps
          </p>
        </div>

        {/* Actions */}
        <div className="flex gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-white border border-gray-300 text-gray-700 rounded hover:bg-gray-50 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleRun}
            disabled={executing}
            className="px-4 py-2 bg-station-blue text-white rounded hover:bg-blue-600 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {executing ? (
              <>
                <div className="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
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
