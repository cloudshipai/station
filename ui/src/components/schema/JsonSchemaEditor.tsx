import React, { useState, useEffect } from 'react';
import Editor from '@monaco-editor/react';
import { Eye, Code, CheckCircle, XCircle, FormInput } from 'lucide-react';
import Form from '@rjsf/core';
import validator from '@rjsf/validator-ajv8';

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
  const [viewMode, setViewMode] = useState<'form' | 'visual' | 'code'>('form');
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
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header with view mode toggle */}
      <div className="flex items-center justify-between p-3 border-b border-tokyo-dark3 bg-tokyo-dark1 flex-shrink-0">
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
              onClick={() => setViewMode('form')}
              className={`px-2 py-1 text-xs rounded transition-colors ${
                viewMode === 'form'
                  ? 'bg-tokyo-blue text-tokyo-bg'
                  : 'text-tokyo-comment hover:text-tokyo-blue'
              }`}
              title="Form Builder"
            >
              <FormInput className="h-3 w-3" />
            </button>
            <button
              onClick={() => setViewMode('visual')}
              className={`px-2 py-1 text-xs rounded transition-colors ${
                viewMode === 'visual'
                  ? 'bg-tokyo-blue text-tokyo-bg'
                  : 'text-tokyo-comment hover:text-tokyo-blue'
              }`}
              title="Visual Preview"
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
              title="Code Editor"
            >
              <Code className="h-3 w-3" />
            </button>
          </div>
        </div>
      </div>

      {/* Validation error display */}
      {validationError && (
        <div className="p-2 bg-tokyo-red bg-opacity-10 border-b border-tokyo-red flex-shrink-0">
          <div className="text-xs text-tokyo-red font-mono">{validationError}</div>
        </div>
      )}

      {/* Content area */}
      <div className="flex-1 overflow-y-auto overflow-x-hidden min-h-0">
        {viewMode === 'form' ? (
          <div className="p-4 bg-tokyo-bg">
            <SchemaFormBuilder
              schemaObject={schemaObject}
              onChange={(newSchema) => onChange(JSON.stringify(newSchema, null, 2))}
            />
          </div>
        ) : viewMode === 'code' ? (
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
      <div className="p-2 border-t border-tokyo-dark3 bg-tokyo-dark1 flex-shrink-0">
        <p className="text-tokyo-comment text-xs font-mono">
          {viewMode === 'form'
            ? '📝 Build schema using form controls'
            : viewMode === 'code'
            ? '💡 Define a JSON schema for agent output validation'
            : '👁️ Visual preview of the schema structure'
          }
        </p>
      </div>
    </div>
  );
};

// Schema Form Builder Component
interface SchemaFormBuilderProps {
  schemaObject: any;
  onChange: (schema: any) => void;
}

const SchemaFormBuilder: React.FC<SchemaFormBuilderProps> = ({ schemaObject, onChange }) => {
  const [properties, setProperties] = useState<any>({});
  const [requiredFields, setRequiredFields] = useState<string[]>([]);

  useEffect(() => {
    if (schemaObject?.properties) {
      setProperties(schemaObject.properties);
      setRequiredFields(schemaObject.required || []);
    }
  }, [schemaObject]);

  const addProperty = () => {
    const newPropName = `field_${Object.keys(properties).length + 1}`;
    setProperties({
      ...properties,
      [newPropName]: { type: 'string', description: '' }
    });
  };

  const removeProperty = (propName: string) => {
    const newProps = { ...properties };
    delete newProps[propName];
    setProperties(newProps);
    setRequiredFields(requiredFields.filter(f => f !== propName));
    updateSchema(newProps, requiredFields.filter(f => f !== propName));
  };

  const updateProperty = (oldName: string, newName: string, config: any) => {
    const newProps = { ...properties };
    if (oldName !== newName) {
      delete newProps[oldName];
      const newRequired = requiredFields.map(f => f === oldName ? newName : f);
      setRequiredFields(newRequired);
    }
    newProps[newName] = config;
    setProperties(newProps);
    updateSchema(newProps, requiredFields);
  };

  const toggleRequired = (propName: string) => {
    const newRequired = requiredFields.includes(propName)
      ? requiredFields.filter(f => f !== propName)
      : [...requiredFields, propName];
    setRequiredFields(newRequired);
    updateSchema(properties, newRequired);
  };

  const updateSchema = (props: any, required: string[]) => {
    const newSchema: any = {
      type: 'object',
      properties: props
    };
    if (required.length > 0) {
      newSchema.required = required;
    }
    onChange(newSchema);
  };

  if (!schemaObject) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <button
          onClick={() => {
            const initialSchema = {
              type: 'object',
              properties: {},
              required: []
            };
            onChange(initialSchema);
          }}
          className="px-4 py-2 bg-tokyo-blue hover:bg-tokyo-blue5 text-tokyo-bg rounded font-mono text-sm transition-colors"
        >
          + Create New Schema
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-mono text-tokyo-blue">Schema Properties</h4>
        <button
          onClick={addProperty}
          className="px-3 py-1 bg-tokyo-green hover:bg-green-600 text-tokyo-bg rounded font-mono text-xs transition-colors"
        >
          + Add Field
        </button>
      </div>

      {Object.entries(properties).map(([propName, config]: [string, any]) => (
        <PropertyEditor
          key={propName}
          name={propName}
          config={config}
          isRequired={requiredFields.includes(propName)}
          onUpdate={(newName, newConfig) => updateProperty(propName, newName, newConfig)}
          onToggleRequired={() => toggleRequired(propName)}
          onRemove={() => removeProperty(propName)}
        />
      ))}

      {Object.keys(properties).length === 0 && (
        <div className="text-tokyo-comment text-sm text-center py-8">
          No properties defined. Click "+ Add Field" to start building your schema.
        </div>
      )}
    </div>
  );
};

// Property Editor Component
interface PropertyEditorProps {
  name: string;
  config: any;
  isRequired: boolean;
  onUpdate: (name: string, config: any) => void;
  onToggleRequired: () => void;
  onRemove: () => void;
}

const PropertyEditor: React.FC<PropertyEditorProps> = ({
  name,
  config,
  isRequired,
  onUpdate,
  onToggleRequired,
  onRemove
}) => {
  const [propName, setPropName] = useState(name);
  const [propType, setPropType] = useState(config.type || 'string');
  const [propDesc, setPropDesc] = useState(config.description || '');
  const [isExpanded, setIsExpanded] = useState(false);
  const [nestedProps, setNestedProps] = useState<any>(config.properties || {});
  const [arrayItemType, setArrayItemType] = useState(config.items?.type || 'string');

  const handleUpdate = () => {
    const newConfig: any = {
      type: propType,
      description: propDesc
    };

    if (propType === 'object' && Object.keys(nestedProps).length > 0) {
      newConfig.properties = nestedProps;
    }

    if (propType === 'array') {
      newConfig.items = { type: arrayItemType };
    }

    onUpdate(propName, newConfig);
  };

  const handleTypeChange = (newType: string) => {
    setPropType(newType);
    if (newType === 'object' || newType === 'array') {
      setIsExpanded(true);
    }
    setTimeout(handleUpdate, 0);
  };

  const addNestedProperty = () => {
    const newNestedName = `nested_${Object.keys(nestedProps).length + 1}`;
    const updated = {
      ...nestedProps,
      [newNestedName]: { type: 'string', description: '' }
    };
    setNestedProps(updated);
    setTimeout(() => {
      onUpdate(propName, {
        type: propType,
        description: propDesc,
        properties: updated
      });
    }, 0);
  };

  const updateNestedProperty = (oldName: string, newName: string, newConfig: any) => {
    const updated = { ...nestedProps };
    if (oldName !== newName) {
      delete updated[oldName];
    }
    updated[newName] = newConfig;
    setNestedProps(updated);
    onUpdate(propName, {
      type: propType,
      description: propDesc,
      properties: updated
    });
  };

  const removeNestedProperty = (nestedName: string) => {
    const updated = { ...nestedProps };
    delete updated[nestedName];
    setNestedProps(updated);
    onUpdate(propName, {
      type: propType,
      description: propDesc,
      properties: updated
    });
  };

  return (
    <div className="bg-tokyo-dark1 rounded border border-tokyo-dark3 p-3 space-y-2">
      <div className="flex items-start gap-2">
        <div className="flex-1 space-y-2">
          <div className="flex items-center gap-2">
            <input
              type="text"
              value={propName}
              onChange={(e) => setPropName(e.target.value)}
              onBlur={handleUpdate}
              className="flex-1 px-2 py-1 bg-tokyo-bg border border-tokyo-dark3 rounded text-tokyo-blue font-mono text-sm focus:outline-none focus:border-tokyo-blue"
              placeholder="field_name"
            />
            <select
              value={propType}
              onChange={(e) => handleTypeChange(e.target.value)}
              className="px-2 py-1 bg-tokyo-bg border border-tokyo-dark3 rounded text-tokyo-green font-mono text-sm focus:outline-none focus:border-tokyo-blue"
            >
              <option value="string">string</option>
              <option value="number">number</option>
              <option value="boolean">boolean</option>
              <option value="array">array</option>
              <option value="object">object</option>
            </select>
            {(propType === 'object' || propType === 'array') && (
              <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="px-2 py-1 bg-tokyo-bg hover:bg-tokyo-dark3 border border-tokyo-dark3 rounded text-tokyo-comment text-xs transition-colors"
              >
                {isExpanded ? '▼' : '▶'}
              </button>
            )}
          </div>
          <input
            type="text"
            value={propDesc}
            onChange={(e) => setPropDesc(e.target.value)}
            onBlur={handleUpdate}
            className="w-full px-2 py-1 bg-tokyo-bg border border-tokyo-dark3 rounded text-tokyo-comment text-xs focus:outline-none focus:border-tokyo-blue"
            placeholder="Description (optional)"
          />
          <label className="flex items-center gap-2 text-xs text-tokyo-comment cursor-pointer">
            <input
              type="checkbox"
              checked={isRequired}
              onChange={onToggleRequired}
              className="rounded"
            />
            <span>Required field</span>
          </label>

          {/* Array item type selector */}
          {propType === 'array' && isExpanded && (
            <div className="pl-4 border-l-2 border-tokyo-purple space-y-2">
              <div className="text-xs text-tokyo-purple font-mono">Array Items:</div>
              <select
                value={arrayItemType}
                onChange={(e) => {
                  setArrayItemType(e.target.value);
                  setTimeout(() => {
                    onUpdate(propName, {
                      type: propType,
                      description: propDesc,
                      items: { type: e.target.value }
                    });
                  }, 0);
                }}
                className="px-2 py-1 bg-tokyo-bg border border-tokyo-dark3 rounded text-tokyo-orange font-mono text-xs focus:outline-none focus:border-tokyo-blue"
              >
                <option value="string">string</option>
                <option value="number">number</option>
                <option value="boolean">boolean</option>
                <option value="object">object</option>
              </select>
            </div>
          )}

          {/* Nested object properties */}
          {propType === 'object' && isExpanded && (
            <div className="pl-4 border-l-2 border-tokyo-orange space-y-2">
              <div className="flex items-center justify-between">
                <div className="text-xs text-tokyo-orange font-mono">Object Properties:</div>
                <button
                  onClick={addNestedProperty}
                  className="px-2 py-0.5 bg-tokyo-orange hover:bg-orange-600 text-tokyo-bg rounded font-mono text-xs transition-colors"
                >
                  + Add
                </button>
              </div>
              {Object.entries(nestedProps).map(([nestedName, nestedConfig]: [string, any]) => (
                <div key={nestedName} className="bg-tokyo-dark2 border border-tokyo-dark3 rounded p-2 space-y-1">
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      defaultValue={nestedName}
                      onBlur={(e) => updateNestedProperty(nestedName, e.target.value, nestedConfig)}
                      className="flex-1 px-2 py-1 bg-tokyo-bg border border-tokyo-blue7 rounded text-tokyo-cyan font-mono text-xs focus:outline-none focus:border-tokyo-orange"
                    />
                    <select
                      value={nestedConfig.type}
                      onChange={(e) => updateNestedProperty(nestedName, nestedName, { ...nestedConfig, type: e.target.value })}
                      className="px-2 py-1 bg-tokyo-bg border border-tokyo-blue7 rounded text-tokyo-green font-mono text-xs focus:outline-none focus:border-tokyo-orange"
                    >
                      <option value="string">string</option>
                      <option value="number">number</option>
                      <option value="boolean">boolean</option>
                      <option value="object">object</option>
                    </select>
                    <button
                      onClick={() => removeNestedProperty(nestedName)}
                      className="px-1.5 py-0.5 bg-tokyo-red hover:bg-red-600 text-tokyo-bg rounded text-xs"
                    >
                      ×
                    </button>
                  </div>
                </div>
              ))}
              {Object.keys(nestedProps).length === 0 && (
                <div className="text-xs text-tokyo-comment text-center py-2">
                  No nested properties
                </div>
              )}
            </div>
          )}
        </div>
        <button
          onClick={onRemove}
          className="px-2 py-1 bg-tokyo-red hover:bg-red-600 text-tokyo-bg rounded text-xs transition-colors"
        >
          Remove
        </button>
      </div>
    </div>
  );
};
