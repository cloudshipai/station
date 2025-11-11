import React, { useState, useEffect } from 'react';
import { X, ArrowRight, ArrowLeft, DollarSign, Shield, Cog, Edit3, AlertTriangle } from 'lucide-react';
import { reportsApi, environmentsApi } from '../../api/station';
import type { Environment, CreateReportRequest } from '../../types/station';

interface CreateReportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (reportId: number) => void;
  defaultEnvironmentId?: number;
}

interface CriterionConfig {
  weight: number;
  threshold: number;
  description: string;
}

type CriteriaTemplate = 'finops' | 'security' | 'devops' | 'custom';

const CRITERIA_TEMPLATES: Record<CriteriaTemplate, {
  goal: string;
  criteria: Record<string, CriterionConfig>;
}> = {
  finops: {
    goal: 'Optimize cloud costs and identify waste',
    criteria: {
      cost_savings_identified: {
        weight: 0.40,
        threshold: 8.0,
        description: 'Dollar value of savings opportunities found',
      },
      accuracy: {
        weight: 0.30,
        threshold: 8.5,
        description: 'Correctness of cost analysis and recommendations',
      },
      actionability: {
        weight: 0.20,
        threshold: 7.5,
        description: 'Clear, implementable recommendations',
      },
      coverage: {
        weight: 0.10,
        threshold: 8.0,
        description: 'Percentage of resources analyzed',
      },
    },
  },
  security: {
    goal: 'Maintain security posture and reduce vulnerability exposure',
    criteria: {
      vulnerability_detection: {
        weight: 0.40,
        threshold: 9.0,
        description: 'CVE detection rate and coverage',
      },
      false_positive_rate: {
        weight: 0.25,
        threshold: 8.0,
        description: 'Accuracy of security findings',
      },
      remediation_speed: {
        weight: 0.20,
        threshold: 7.5,
        description: 'Time to identify and fix issues',
      },
      compliance: {
        weight: 0.15,
        threshold: 9.5,
        description: 'Regulatory compliance adherence',
      },
    },
  },
  devops: {
    goal: 'Ensure infrastructure reliability and deployment velocity',
    criteria: {
      availability: {
        weight: 0.35,
        threshold: 9.5,
        description: 'System uptime and availability',
      },
      deployment_success: {
        weight: 0.30,
        threshold: 9.0,
        description: 'Successful deployment rate',
      },
      recovery_time: {
        weight: 0.20,
        threshold: 8.0,
        description: 'MTTR for incidents',
      },
      efficiency: {
        weight: 0.15,
        threshold: 7.5,
        description: 'Resource utilization',
      },
    },
  },
  custom: {
    goal: '',
    criteria: {},
  },
};

export const CreateReportModal: React.FC<CreateReportModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  defaultEnvironmentId,
}) => {
  const [step, setStep] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Step 1: Basic Info
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [environmentId, setEnvironmentId] = useState<number | null>(defaultEnvironmentId || null);
  const [environments, setEnvironments] = useState<Environment[]>([]);

  // Step 2: Criteria
  const [selectedTemplate, setSelectedTemplate] = useState<CriteriaTemplate>('finops');
  const [goal, setGoal] = useState(CRITERIA_TEMPLATES.finops.goal);
  const [criteria, setCriteria] = useState<Record<string, CriterionConfig>>(
    CRITERIA_TEMPLATES.finops.criteria
  );

  // Load environments
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        setEnvironments(response.data.environments || []);
        if (!environmentId && response.data.environments.length > 0) {
          setEnvironmentId(response.data.environments[0].id);
        }
      } catch (err) {
        console.error('Failed to fetch environments:', err);
      }
    };
    if (isOpen) {
      fetchEnvironments();
    }
  }, [isOpen]);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setStep(1);
      setName('');
      setDescription('');
      setEnvironmentId(defaultEnvironmentId || null);
      setSelectedTemplate('finops');
      setGoal(CRITERIA_TEMPLATES.finops.goal);
      setCriteria(CRITERIA_TEMPLATES.finops.criteria);
      setError(null);
    }
  }, [isOpen, defaultEnvironmentId]);

  const handleTemplateChange = (template: CriteriaTemplate) => {
    setSelectedTemplate(template);
    setGoal(CRITERIA_TEMPLATES[template].goal);
    setCriteria({ ...CRITERIA_TEMPLATES[template].criteria });
  };

  const handleCriterionChange = (
    criterionName: string,
    field: 'weight' | 'threshold' | 'description',
    value: number | string
  ) => {
    setCriteria(prev => ({
      ...prev,
      [criterionName]: {
        ...prev[criterionName],
        [field]: field === 'description' ? value : parseFloat(value as string),
      },
    }));
  };

  const addCustomCriterion = () => {
    const newName = `criterion_${Object.keys(criteria).length + 1}`;
    setCriteria(prev => ({
      ...prev,
      [newName]: {
        weight: 0.1,
        threshold: 7.0,
        description: 'New criterion',
      },
    }));
  };

  const removeCriterion = (criterionName: string) => {
    const { [criterionName]: removed, ...rest } = criteria;
    setCriteria(rest);
  };

  const getTotalWeight = () => {
    return Object.values(criteria).reduce((sum, c) => sum + c.weight, 0);
  };

  const isWeightValid = () => {
    const total = getTotalWeight();
    return Math.abs(total - 1.0) < 0.001;
  };

  const canProceedToStep2 = () => {
    return name.trim() !== '' && environmentId !== null;
  };

  const canProceedToStep3 = () => {
    return (
      goal.trim() !== '' &&
      Object.keys(criteria).length > 0 &&
      isWeightValid()
    );
  };

  const handleCreate = async () => {
    if (!environmentId) {
      setError('Please select an environment');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const request: CreateReportRequest = {
        name,
        description: description || undefined,
        environment_id: environmentId,
        team_criteria: {
          goal,
          criteria,
        },
        judge_model: 'gpt-4o-mini',
      };

      const response = await reportsApi.create(request);
      const reportId = response.data.report.id;

      // Automatically trigger generation
      await reportsApi.generate(reportId);

      onSuccess(reportId);
      onClose();
    } catch (err: any) {
      console.error('Failed to create report:', err);
      setError(err.response?.data?.error || 'Failed to create report');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="sticky top-0 bg-tokyo-bg-dark border-b border-tokyo-dark3 p-6 flex items-center justify-between z-10">
          <div>
            <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
              Create New Report
            </h2>
            <p className="text-sm text-tokyo-comment font-mono mt-1">
              Step {step}/3
            </p>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        {/* Progress */}
        <div className="px-6 pt-4">
          <div className="flex items-center gap-2">
            {[1, 2, 3].map((s) => (
              <div
                key={s}
                className={`flex-1 h-2 rounded-full ${
                  s <= step ? 'bg-tokyo-blue' : 'bg-tokyo-dark3'
                }`}
              />
            ))}
          </div>
        </div>

        {/* Content */}
        <div className="p-6">
          {error && (
            <div className="mb-4 p-4 bg-tokyo-red/20 border border-tokyo-red rounded flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-tokyo-red" />
              <span className="text-tokyo-red font-mono text-sm">{error}</span>
            </div>
          )}

          {/* Step 1: Basic Info */}
          {step === 1 && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-mono text-tokyo-fg mb-2">
                  Report Name *
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Q1 2025 Cost Optimization Review"
                  className="w-full px-4 py-2 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 focus:border-tokyo-blue focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-sm font-mono text-tokyo-fg mb-2">
                  Description (optional)
                </label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Comprehensive evaluation of all cost-saving agents..."
                  rows={3}
                  className="w-full px-4 py-2 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 focus:border-tokyo-blue focus:outline-none resize-none"
                />
              </div>

              <div>
                <label className="block text-sm font-mono text-tokyo-fg mb-2">
                  Environment *
                </label>
                <select
                  value={environmentId || ''}
                  onChange={(e) => setEnvironmentId(parseInt(e.target.value))}
                  className="w-full px-4 py-2 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 focus:border-tokyo-blue focus:outline-none"
                >
                  <option value="">Select environment</option>
                  {environments.map((env) => (
                    <option key={env.id} value={env.id}>
                      {env.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          )}

          {/* Step 2: Criteria */}
          {step === 2 && (
            <div className="space-y-6">
              {/* Template Selection */}
              <div>
                <label className="block text-sm font-mono text-tokyo-fg mb-3">
                  Choose a Template
                </label>
                <div className="grid grid-cols-4 gap-3">
                  {[
                    { id: 'finops', icon: DollarSign, label: 'FinOps' },
                    { id: 'security', icon: Shield, label: 'Security' },
                    { id: 'devops', icon: Cog, label: 'DevOps' },
                    { id: 'custom', icon: Edit3, label: 'Custom' },
                  ].map((template) => {
                    const Icon = template.icon;
                    const isSelected = selectedTemplate === template.id;
                    return (
                      <button
                        key={template.id}
                        onClick={() => handleTemplateChange(template.id as CriteriaTemplate)}
                        className={`p-4 border-2 rounded-lg transition-all ${
                          isSelected
                            ? 'border-tokyo-blue bg-tokyo-blue/20'
                            : 'border-tokyo-dark3 hover:border-tokyo-blue5'
                        }`}
                      >
                        <Icon className={`h-8 w-8 mx-auto mb-2 ${isSelected ? 'text-tokyo-blue' : 'text-tokyo-comment'}`} />
                        <div className={`text-sm font-mono ${isSelected ? 'text-tokyo-blue font-semibold' : 'text-tokyo-fg'}`}>
                          {template.label}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Business Goal */}
              <div>
                <label className="block text-sm font-mono text-tokyo-fg mb-2">
                  Business Goal *
                </label>
                <input
                  type="text"
                  value={goal}
                  onChange={(e) => setGoal(e.target.value)}
                  placeholder="Reduce AWS costs by 20% in Q1 2025"
                  className="w-full px-4 py-2 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 focus:border-tokyo-blue focus:outline-none"
                />
              </div>

              {/* Criteria List */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <label className="text-sm font-mono text-tokyo-fg">
                    Evaluation Criteria (must sum to 100%)
                  </label>
                  <div className={`text-sm font-mono ${isWeightValid() ? 'text-tokyo-green' : 'text-tokyo-red'}`}>
                    Total: {(getTotalWeight() * 100).toFixed(0)}%
                  </div>
                </div>

                <div className="space-y-3">
                  {Object.entries(criteria).map(([name, config]) => (
                    <div key={name} className="p-4 bg-tokyo-bg border border-tokyo-dark3 rounded">
                      <div className="flex items-start justify-between mb-3">
                        <div className="flex-1">
                          <input
                            type="text"
                            value={name}
                            disabled
                            className="font-mono text-sm text-tokyo-fg bg-transparent capitalize mb-1"
                          />
                          <input
                            type="text"
                            value={config.description}
                            onChange={(e) =>
                              handleCriterionChange(name, 'description', e.target.value)
                            }
                            className="w-full text-xs font-mono text-tokyo-comment bg-tokyo-bg-dark px-2 py-1 rounded border border-tokyo-dark3"
                          />
                        </div>
                        {Object.keys(criteria).length > 1 && (
                          <button
                            onClick={() => removeCriterion(name)}
                            className="ml-2 text-tokyo-red hover:text-red-400 text-xs"
                          >
                            Remove
                          </button>
                        )}
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs font-mono text-tokyo-comment mb-1 block">
                            Weight
                          </label>
                          <div className="flex items-center gap-2">
                            <input
                              type="range"
                              min="0"
                              max="1"
                              step="0.05"
                              value={config.weight}
                              onChange={(e) =>
                                handleCriterionChange(name, 'weight', e.target.value)
                              }
                              className="flex-1"
                            />
                            <span className="text-xs font-mono text-tokyo-fg w-12">
                              {(config.weight * 100).toFixed(0)}%
                            </span>
                          </div>
                        </div>

                        <div>
                          <label className="text-xs font-mono text-tokyo-comment mb-1 block">
                            Threshold
                          </label>
                          <div className="flex items-center gap-2">
                            <input
                              type="range"
                              min="0"
                              max="10"
                              step="0.5"
                              value={config.threshold}
                              onChange={(e) =>
                                handleCriterionChange(name, 'threshold', e.target.value)
                              }
                              className="flex-1"
                            />
                            <span className="text-xs font-mono text-tokyo-fg w-12">
                              {config.threshold.toFixed(1)}/10
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>

                <button
                  onClick={addCustomCriterion}
                  className="mt-3 px-4 py-2 bg-tokyo-dark2 border border-tokyo-dark3 text-tokyo-fg hover:border-tokyo-blue5 rounded font-mono text-sm transition-colors"
                >
                  + Add Custom Criterion
                </button>
              </div>
            </div>
          )}

          {/* Step 3: Review */}
          {step === 3 && (
            <div className="space-y-4">
              <div className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                <h3 className="text-sm font-mono font-semibold text-tokyo-cyan mb-3">
                  Report Summary
                </h3>
                <div className="space-y-2 text-sm font-mono">
                  <div>
                    <span className="text-tokyo-comment">Name:</span>{' '}
                    <span className="text-tokyo-fg">{name}</span>
                  </div>
                  {description && (
                    <div>
                      <span className="text-tokyo-comment">Description:</span>{' '}
                      <span className="text-tokyo-fg">{description}</span>
                    </div>
                  )}
                  <div>
                    <span className="text-tokyo-comment">Environment:</span>{' '}
                    <span className="text-tokyo-fg">
                      {environments.find((e) => e.id === environmentId)?.name}
                    </span>
                  </div>
                  <div>
                    <span className="text-tokyo-comment">Goal:</span>{' '}
                    <span className="text-tokyo-fg">{goal}</span>
                  </div>
                </div>
              </div>

              <div className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                <h3 className="text-sm font-mono font-semibold text-tokyo-cyan mb-3">
                  Evaluation Criteria
                </h3>
                <ul className="space-y-2 text-sm font-mono">
                  {Object.entries(criteria).map(([name, config]) => (
                    <li key={name} className="flex items-center justify-between">
                      <span className="text-tokyo-fg capitalize">
                        {name.replace(/_/g, ' ')}
                      </span>
                      <span className="text-tokyo-comment">
                        {(config.weight * 100).toFixed(0)}% weight, {config.threshold.toFixed(1)}/10 threshold
                      </span>
                    </li>
                  ))}
                </ul>
              </div>

              <div className="p-4 bg-tokyo-blue/10 border border-tokyo-blue rounded">
                <h3 className="text-sm font-mono font-semibold text-tokyo-blue mb-2">
                  Estimated Generation
                </h3>
                <div className="space-y-1 text-xs font-mono text-tokyo-comment">
                  <div>⏱️ Duration: ~26 seconds (with parallel evaluation)</div>
                  <div>💰 LLM Cost: ~$0.014 (GPT-4o-mini)</div>
                  <div>🧠 Judge Model: gpt-4o-mini</div>
                </div>
              </div>

              <div className="p-3 bg-tokyo-yellow/10 border border-tokyo-yellow rounded">
                <p className="text-xs font-mono text-tokyo-comment">
                  ⚠️ Report generation runs in the background. You can view progress from the
                  reports list.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="sticky bottom-0 bg-tokyo-bg-dark border-t border-tokyo-dark3 p-6 flex items-center justify-between">
          <div>
            {step > 1 && (
              <button
                onClick={() => setStep(step - 1)}
                disabled={loading}
                className="flex items-center gap-2 px-4 py-2 text-tokyo-fg hover:text-tokyo-blue font-mono text-sm transition-colors disabled:opacity-50"
              >
                <ArrowLeft className="h-4 w-4" />
                Back
              </button>
            )}
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              disabled={loading}
              className="px-4 py-2 text-tokyo-comment hover:text-tokyo-fg font-mono text-sm transition-colors disabled:opacity-50"
            >
              Cancel
            </button>

            {step < 3 ? (
              <button
                onClick={() => setStep(step + 1)}
                disabled={
                  loading ||
                  (step === 1 && !canProceedToStep2()) ||
                  (step === 2 && !canProceedToStep3())
                }
                className="flex items-center gap-2 px-4 py-2 bg-tokyo-blue text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Next: {step === 1 ? 'Criteria' : 'Review'}
                <ArrowRight className="h-4 w-4" />
              </button>
            ) : (
              <button
                onClick={handleCreate}
                disabled={loading}
                className="px-4 py-2 bg-tokyo-green text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {loading ? 'Creating...' : 'Create & Generate Report'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
