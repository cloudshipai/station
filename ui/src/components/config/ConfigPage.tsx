import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { apiClient } from '../../api/client';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../ui/card';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { AlertCircle, CheckCircle, Loader2, Save, X, Settings, Eye, EyeOff, ChevronDown, ChevronRight, Terminal } from 'lucide-react';

interface ShowWhenCondition {
  field: string;
  values: string[];
}

interface ConfigField {
  key: string;
  type: 'string' | 'int' | 'bool' | '[]string';
  description: string;
  default?: any;
  options?: string[];
  secret?: boolean;
  section: string;
  showWhen?: ShowWhenCondition;
}

interface ConfigSection {
  name: string;
  description: string;
  order: number;
}

interface SchemaResponse {
  sections: ConfigSection[];
  fields: ConfigField[];
}

interface SessionResponse {
  id: string;
  status: string;
  config_path: string;
  values: Record<string, any>;
}

export const ConfigPage: React.FC = () => {
  const [searchParams] = useSearchParams();
  const sessionId = searchParams.get('session_id');
  
  const [schema, setSchema] = useState<SchemaResponse | null>(null);
  const [values, setValues] = useState<Record<string, any>>({});
  const [configPath, setConfigPath] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [completed, setCompleted] = useState(false);
  const [cancelled, setCancelled] = useState(false);
  const [showSecrets, setShowSecrets] = useState<Record<string, boolean>>({});
  const [collapsedSections, setCollapsedSections] = useState<Record<string, boolean>>({});

  useEffect(() => {
    const fetchData = async () => {
      if (!sessionId) {
        setError('No session ID provided. Run: stn config --browser');
        setLoading(false);
        return;
      }

      try {
        const [schemaRes, sessionRes] = await Promise.all([
          apiClient.get<SchemaResponse>('/config/schema'),
          apiClient.get<SessionResponse>(`/config/session/${sessionId}`)
        ]);

        setSchema(schemaRes.data);
        setValues(sessionRes.data.values || {});
        setConfigPath(sessionRes.data.config_path);
      } catch (err: any) {
        console.error('Failed to load config data:', err);
        setError(err.response?.data?.error || 'Failed to load configuration session');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [sessionId]);

  const getNestedValue = (obj: Record<string, any>, key: string): any => {
    return key.split('.').reduce((o, k) => (o && o[k] !== undefined) ? o[k] : undefined, obj);
  };

  const setNestedValue = (obj: Record<string, any>, key: string, value: any): Record<string, any> => {
    const result = { ...obj };
    const parts = key.split('.');
    let current: any = result;
    
    for (let i = 0; i < parts.length - 1; i++) {
      if (!current[parts[i]]) {
        current[parts[i]] = {};
      } else {
        current[parts[i]] = { ...current[parts[i]] };
      }
      current = current[parts[i]];
    }
    
    if (value === '' || value === undefined || value === null) {
      delete current[parts[parts.length - 1]];
    } else {
      current[parts[parts.length - 1]] = value;
    }
    
    return result;
  };

  const handleInputChange = (key: string, value: any, fieldType: string) => {
    let processedValue = value;
    
    if (fieldType === 'int' && value !== '') {
      processedValue = parseInt(value, 10);
      if (isNaN(processedValue)) return;
    } else if (fieldType === '[]string') {
      processedValue = value ? value.split(',').map((s: string) => s.trim()).filter(Boolean) : [];
    }
    
    setValues(prev => setNestedValue(prev, key, processedValue));
  };

  const toggleSecretVisibility = (key: string) => {
    setShowSecrets(prev => ({ ...prev, [key]: !prev[key] }));
  };

  const toggleSection = (sectionName: string) => {
    setCollapsedSections(prev => ({ ...prev, [sectionName]: !prev[sectionName] }));
  };

  const shouldShowField = (field: ConfigField): boolean => {
    if (!field.showWhen) return true;
    
    const dependentValue = getNestedValue(values, field.showWhen.field);
    if (dependentValue === undefined) {
      const dependentField = schema?.fields.find(f => f.key === field.showWhen!.field);
      const defaultValue = dependentField?.default;
      return field.showWhen.values.includes(String(defaultValue ?? ''));
    }
    return field.showWhen.values.includes(String(dependentValue));
  };

  const handleSave = async () => {
    if (!sessionId) return;
    
    setSaving(true);
    setError(null);
    
    try {
      await apiClient.post(`/config/session/${sessionId}/save`, { values });
      setCompleted(true);
    } catch (err: any) {
      console.error('Failed to save config:', err);
      setError(err.response?.data?.error || 'Failed to save configuration');
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = async () => {
    if (!sessionId) return;
    
    try {
      await apiClient.delete(`/config/session/${sessionId}`);
    } catch (err: any) {
      console.error('Failed to cancel session:', err);
    }
    setCancelled(true);
  };

  if (!sessionId) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
        <Card className="w-full max-w-md shadow-station-lg">
          <CardContent className="pt-6 text-center">
            <AlertCircle className="h-12 w-12 text-red-500 mx-auto mb-4" />
            <h1 className="text-xl font-bold text-gray-900 mb-2">Configuration Error</h1>
            <p className="text-gray-600">No session ID provided.</p>
            <p className="text-sm text-gray-500 mt-2">Run: <code className="bg-gray-100 px-2 py-1 rounded">stn config --browser</code></p>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (completed) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-green-50 to-blue-50 flex items-center justify-center p-4">
        <Card className="w-full max-w-lg shadow-station-lg">
          <CardContent className="pt-10 pb-10 text-center">
            <div className="w-20 h-20 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-6">
              <CheckCircle className="h-10 w-10 text-green-600" />
            </div>
            <h1 className="text-2xl font-bold text-gray-900 mb-3">Configuration Saved!</h1>
            <p className="text-gray-600 mb-6 text-lg">
              Your settings have been saved to <code className="bg-gray-100 px-2 py-1 rounded text-sm">{configPath}</code>
            </p>
            <div className="bg-gray-50 rounded-lg p-4 border border-gray-100">
              <div className="flex items-center justify-center gap-2 text-gray-700">
                <Terminal className="h-5 w-5" />
                <span>You can close this window and return to the terminal.</span>
              </div>
            </div>
            <p className="text-sm text-gray-500 mt-4">
              The CLI will automatically detect completion and continue.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (cancelled) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
        <Card className="w-full max-w-lg shadow-station-lg">
          <CardContent className="pt-10 pb-10 text-center">
            <div className="w-20 h-20 bg-gray-100 rounded-full flex items-center justify-center mx-auto mb-6">
              <X className="h-10 w-10 text-gray-500" />
            </div>
            <h1 className="text-2xl font-bold text-gray-900 mb-3">Configuration Cancelled</h1>
            <p className="text-gray-600 mb-6">No changes were made to your configuration.</p>
            <div className="bg-gray-50 rounded-lg p-4 border border-gray-100">
              <div className="flex items-center justify-center gap-2 text-gray-700">
                <Terminal className="h-5 w-5" />
                <span>You can close this window and return to the terminal.</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <Loader2 className="h-8 w-8 text-primary animate-spin mx-auto mb-4" />
          <p className="text-gray-600 font-medium">Loading configuration...</p>
        </div>
      </div>
    );
  }

  const fieldsBySection: Record<string, ConfigField[]> = {};
  schema?.fields.forEach(field => {
    if (!fieldsBySection[field.section]) {
      fieldsBySection[field.section] = [];
    }
    fieldsBySection[field.section].push(field);
  });

  const getVisibleFields = (sectionFields: ConfigField[]): ConfigField[] => {
    return sectionFields.filter(shouldShowField);
  };

  const sortedSections = schema?.sections.sort((a, b) => a.order - b.order) || [];

  return (
    <div className="min-h-screen bg-gray-50/50 pb-20">
      <div className="bg-white border-b border-gray-200 sticky top-0 z-10 shadow-sm">
        <div className="max-w-4xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-primary/10 rounded-lg">
              <Settings className="h-6 w-6 text-primary" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-900">Station Configuration</h1>
              <p className="text-xs text-gray-500">{configPath}</p>
            </div>
          </div>
          <div className="flex gap-3">
            <Button variant="ghost" onClick={handleCancel}>
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={saving} className="min-w-[120px]">
              {saving ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Save className="mr-2 h-4 w-4" />
                  Save Changes
                </>
              )}
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-4xl mx-auto px-6 py-8 space-y-4">
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-start gap-3 text-red-700">
            <AlertCircle className="h-5 w-5 flex-shrink-0 mt-0.5" />
            <div>
              <p className="font-medium">Error</p>
              <p className="text-sm opacity-90">{error}</p>
            </div>
          </div>
        )}

        {sortedSections.map((section) => {
          const sectionFields = fieldsBySection[section.name] || [];
          const visibleFields = getVisibleFields(sectionFields);
          if (visibleFields.length === 0) return null;
          
          const isCollapsed = collapsedSections[section.name];
          
          return (
            <Card key={section.name} className="shadow-station border-gray-200/60 overflow-hidden">
              <CardHeader 
                className="bg-gray-50/50 border-b border-gray-100 pb-4 cursor-pointer select-none"
                onClick={() => toggleSection(section.name)}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {isCollapsed ? (
                      <ChevronRight className="h-5 w-5 text-gray-400" />
                    ) : (
                      <ChevronDown className="h-5 w-5 text-gray-400" />
                    )}
                    <CardTitle className="text-lg text-gray-900">
                      {section.description}
                    </CardTitle>
                  </div>
                  <span className="text-xs text-gray-400">{visibleFields.length} settings</span>
                </div>
              </CardHeader>
              
              {!isCollapsed && (
                <CardContent className="pt-6 grid gap-5">
                  {visibleFields.map((field) => {
                    const currentValue = getNestedValue(values, field.key);
                    const displayValue = field.type === '[]string' && Array.isArray(currentValue) 
                      ? currentValue.join(', ') 
                      : currentValue;
                    
                    return (
                      <div key={field.key} className="grid gap-1.5">
                        <Label htmlFor={field.key} className="text-sm font-medium text-gray-700 flex items-center gap-2">
                          <code className="text-xs bg-gray-100 px-1.5 py-0.5 rounded text-gray-600">{field.key}</code>
                        </Label>
                        
                        {field.options && field.options.length > 0 ? (
                          <Select 
                            value={String(displayValue ?? field.default ?? '__none__')} 
                            onValueChange={(val) => handleInputChange(field.key, val === '__none__' ? '' : val, field.type)}
                          >
                            <SelectTrigger id={field.key} className="w-full bg-white">
                              <SelectValue placeholder="Select..." />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="__none__">-- Select --</SelectItem>
                              {field.options.map((opt) => (
                                <SelectItem key={opt} value={opt}>
                                  {opt}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : field.type === 'bool' ? (
                          <div className="flex items-center gap-2">
                            <input
                              type="checkbox"
                              id={field.key}
                              checked={currentValue ?? field.default ?? false}
                              onChange={(e) => handleInputChange(field.key, e.target.checked, field.type)}
                              className="h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary"
                            />
                            <span className="text-sm text-gray-600">{field.description}</span>
                          </div>
                        ) : (
                          <div className="relative">
                            <Input
                              id={field.key}
                              type={field.secret && !showSecrets[field.key] ? 'password' : field.type === 'int' ? 'number' : 'text'}
                              value={displayValue ?? ''}
                              onChange={(e) => handleInputChange(field.key, e.target.value, field.type)}
                              placeholder={field.default !== undefined ? `Default: ${field.default}` : field.description}
                              className="bg-white pr-10"
                            />
                            {field.secret && (
                              <button
                                type="button"
                                onClick={() => toggleSecretVisibility(field.key)}
                                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors"
                              >
                                {showSecrets[field.key] ? (
                                  <EyeOff className="h-4 w-4" />
                                ) : (
                                  <Eye className="h-4 w-4" />
                                )}
                              </button>
                            )}
                          </div>
                        )}
                        
                        {field.type !== 'bool' && (
                          <p className="text-xs text-gray-500">
                            {field.description}
                            {field.default !== undefined && !field.options && (
                              <span className="text-gray-400"> (default: {String(field.default)})</span>
                            )}
                          </p>
                        )}
                      </div>
                    );
                  })}
                </CardContent>
              )}
            </Card>
          );
        })}
      </div>
    </div>
  );
};
