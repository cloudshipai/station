import React, { useState, useEffect } from 'react';
import Editor from '@monaco-editor/react';
import { Eye, Code, CheckCircle, XCircle } from 'lucide-react';

interface JsonSchemaEditorProps {
  schema: string;
  onChange: (schema: string) => void;
  onValidation?: (isValid: boolean, error?: string) => void;
}

export const JsonSchemaEditor: React.FC<JsonSchemaEditorProps> = ({
  schema,
  onChange,
  onValidation
}) => {
  const [viewMode, setViewMode] = useState<'visual' | 'code'>('code');
  const [validationError, setValidationError] = useState<string | null>(null);
  const [schemaObject, setSchemaObject] = useState<any>(null);

  useEffect(() => {
    // Validate and parse schema whenever it changes
    try {
      if (!schema || schema.trim() === '') {
        setSchemaObject(null);
        setValidationError(null);
        onValidation?.(true);
        return;
      }

      const parsed = JSON.parse(schema);
      setSchemaObject(parsed);
      setValidationError(null);
      onValidation?.(true);
    } catch (err: any) {
      setValidationError(err.message);
      setSchemaObject(null);
      onValidation?.(false, err.message);
    }
  }, [schema, onValidation]);

  const formatSchema = () => {
    try {
      const parsed = JSON.parse(schema);
      const formatted = JSON.stringify(parsed, null, 2);
      onChange(formatted);
    } catch (err) {
      // Ignore formatting errors
    }
  };

  const renderSchemaPreview = (obj: any, level = 0): JSX.Element | null => {
    if (!obj) return null;

    const indent = level * 16;

    if (obj.type === 'object' && obj.properties) {
      return (
        <div style={{ marginLeft: indent }} className="my-2">
          <div className="text-tokyo-blue font-mono text-sm">{'{'}</div>
          {Object.entries(obj.properties).map(([key, value]: [string, any]) => (
            <div key={key} className="ml-4 my-1">
              <div className="flex items-start gap-2">
                <span className="text-tokyo-orange font-mono">{key}:</span>
                <div className="flex-1">
                  {value.type && (
                    <span className="text-tokyo-green text-xs bg-tokyo-bg px-1 rounded">
                      {value.type}
                    </span>
                  )}
                  {value.description && (
                    <div className="text-tokyo-comment text-xs mt-1">
                      {value.description}
                    </div>
                  )}
                  {obj.required?.includes(key) && (
                    <span className="text-tokyo-red text-xs ml-2">required</span>
                  )}
                  {value.properties && renderSchemaPreview(value, level + 1)}
                  {value.items && (
                    <div className="ml-4">
                      <span className="text-tokyo-comment text-xs">Array of:</span>
                      {renderSchemaPreview(value.items, level + 1)}
                    </div>
                  )}
                </div>
              </div>
            </div>
          ))}
          <div className="text-tokyo-blue font-mono text-sm">{'}'}</div>
        </div>
      );
    }

    if (obj.type === 'array' && obj.items) {
      return (
        <div style={{ marginLeft: indent }} className="my-2">
          <div className="text-tokyo-comment text-sm">Array of:</div>
          {renderSchemaPreview(obj.items, level + 1)}
        </div>
      );
    }

    return (
      <div style={{ marginLeft: indent }} className="text-tokyo-green text-sm">
        {obj.type || 'any'}
        {obj.description && (
          <div className="text-tokyo-comment text-xs mt-1">{obj.description}</div>
        )}
      </div>
    );
  };

  return (
    <div className="flex flex-col h-full">
      {/* Header with view mode toggle */}
      <div className="flex items-center justify-between p-3 border-b border-tokyo-dark3 bg-tokyo-dark1">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-mono text-tokyo-blue font-medium">Output Schema</h3>
          {validationError ? (
            <XCircle className="h-4 w-4 text-tokyo-red" />
          ) : schema ? (
            <CheckCircle className="h-4 w-4 text-tokyo-green" />
          ) : null}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={formatSchema}
            disabled={!schema || !!validationError}
            className="px-2 py-1 text-xs bg-tokyo-bg hover:bg-tokyo-dark3 text-tokyo-comment rounded transition-colors disabled:opacity-50"
          >
            Format
          </button>
          <div className="flex items-center gap-1 bg-tokyo-bg rounded">
            <button
              onClick={() => setViewMode('visual')}
              className={`px-2 py-1 text-xs rounded transition-colors ${
                viewMode === 'visual'
                  ? 'bg-tokyo-blue text-tokyo-bg'
                  : 'text-tokyo-comment hover:text-tokyo-blue'
              }`}
            >
              <Eye className="h-3 w-3" />
            </button>
            <button
              onClick={() => setViewMode('code')}
              className={`px-2 py-1 text-xs rounded transition-colors ${
                viewMode === 'code'
                  ? 'bg-tokyo-blue text-tokyo-bg'
                  : 'text-tokyo-comment hover:text-tokyo-blue'
              }`}
            >
              <Code className="h-3 w-3" />
            </button>
          </div>
        </div>
      </div>

      {/* Validation error display */}
      {validationError && (
        <div className="p-2 bg-tokyo-red bg-opacity-10 border-b border-tokyo-red">
          <div className="text-xs text-tokyo-red font-mono">{validationError}</div>
        </div>
      )}

      {/* Content area */}
      <div className="flex-1 overflow-hidden">
        {viewMode === 'code' ? (
          <Editor
            height="100%"
            defaultLanguage="json"
            value={schema}
            onChange={(value) => onChange(value || '')}
            theme="vs-dark"
            options={{
              minimap: { enabled: false },
              fontSize: 12,
              fontFamily: 'JetBrains Mono, Fira Code, Monaco, monospace',
              lineNumbers: 'on',
              wordWrap: 'on',
              automaticLayout: true,
              scrollBeyondLastLine: false,
              padding: { top: 8, bottom: 8 },
              formatOnPaste: true,
              formatOnType: true
            }}
          />
        ) : (
          <div className="h-full overflow-auto p-4 bg-tokyo-bg">
            {schemaObject ? (
              <div>
                <div className="text-tokyo-comment text-xs mb-3 font-mono">
                  Visual schema representation
                </div>
                {renderSchemaPreview(schemaObject)}
              </div>
            ) : (
              <div className="text-tokyo-comment text-sm text-center mt-8">
                {schema ? 'Invalid JSON schema' : 'No schema defined'}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Helper text */}
      <div className="p-2 border-t border-tokyo-dark3 bg-tokyo-dark1">
        <p className="text-tokyo-comment text-xs font-mono">
          {viewMode === 'code'
            ? '💡 Define a JSON schema for agent output validation'
            : '👁️ Visual preview of the schema structure'
          }
        </p>
      </div>
    </div>
  );
};
