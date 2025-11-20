import React, { useState, useEffect, useRef } from 'react';
import { X, Play, Loader, CheckCircle, XCircle, Zap, TrendingUp, Clock, DollarSign, AlertTriangle } from 'lucide-react';
import { agentsApi, agentRunsApi, benchmarksApi } from '../../api/station';
import type { Agent, AgentRun } from '../../types/station';

interface BenchmarkExperimentModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentId?: number;
  onComplete?: (results: ExperimentResults) => void;
}

interface ExperimentConfig {
  selectedAgents: number[];
  selectedRuns: number[];
  concurrency: number;
  includeAllRuns: boolean;
}

interface RunProgress {
  run_id: number;
  status: 'pending' | 'evaluating' | 'completed' | 'failed';
  quality_score?: number;
  production_ready?: boolean;
  error?: string;
  evaluated_at?: string;
}

interface ExperimentResults {
  totalRuns: number;
  completed: number;
  failed: number;
  durationSeconds: number;
  avgQualityScore: number;
  productionReadyPct: number;
  results: RunProgress[];
}

export const BenchmarkExperimentModal: React.FC<BenchmarkExperimentModalProps> = ({
  isOpen,
  onClose,
  environmentId,
  onComplete,
}) => {
  const [step, setStep] = useState<'config' | 'running' | 'results'>('config');
  const [agents, setAgents] = useState<Agent[]>([]);
  const [agentRuns, setAgentRuns] = useState<Record<number, AgentRun[]>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const [config, setConfig] = useState<ExperimentConfig>({
    selectedAgents: [],
    selectedRuns: [],
    concurrency: 10,
    includeAllRuns: false,
  });

  const [runProgress, setRunProgress] = useState<Map<number, RunProgress>>(new Map());
  const [experimentResults, setExperimentResults] = useState<ExperimentResults | null>(null);
  const [startTime, setStartTime] = useState<Date | null>(null);
  const [elapsedTime, setElapsedTime] = useState(0);

  const eventSourceRef = useRef<EventSource | null>(null);
  const timerRef = useRef<NodeJS.Timeout | null>(null);

  // Fetch agents when modal opens
  useEffect(() => {
    if (isOpen && agents.length === 0) {
      fetchAgents();
    }
  }, [isOpen]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, []);

  // Timer for elapsed time
  useEffect(() => {
    if (step === 'running' && startTime) {
      timerRef.current = setInterval(() => {
        setElapsedTime(Math.floor((Date.now() - startTime.getTime()) / 1000));
      }, 100);
    } else {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    }

    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, [step, startTime]);

  const fetchAgents = async () => {
    setLoading(true);
    try {
      const params = environmentId ? { environment_id: environmentId } : {};
      const response = await agentsApi.getAll(params);
      const agentsList = response.data.agents || [];
      setAgents(agentsList);

      // Fetch runs for each agent
      const runsMap: Record<number, AgentRun[]> = {};
      await Promise.all(
        agentsList.map(async (agent) => {
          try {
            const runsResponse = await agentRunsApi.getByAgent(agent.id);
            runsMap[agent.id] = runsResponse.data.runs || [];
          } catch {
            runsMap[agent.id] = [];
          }
        })
      );
      setAgentRuns(runsMap);
    } catch (err) {
      console.error('Failed to fetch agents:', err);
      setError('Failed to load agents');
    } finally {
      setLoading(false);
    }
  };

  const toggleAgent = (agentId: number) => {
    setConfig((prev) => {
      const selectedAgents = prev.selectedAgents.includes(agentId)
        ? prev.selectedAgents.filter((id) => id !== agentId)
        : [...prev.selectedAgents, agentId];

      // Update selected runs if includeAllRuns is true
      let selectedRuns = prev.selectedRuns;
      if (prev.includeAllRuns) {
        selectedRuns = selectedAgents.flatMap((id) => 
          agentRuns[id]?.map((r) => r.id) || []
        );
      }

      return { ...prev, selectedAgents, selectedRuns };
    });
  };

  const toggleRun = (runId: number) => {
    setConfig((prev) => ({
      ...prev,
      selectedRuns: prev.selectedRuns.includes(runId)
        ? prev.selectedRuns.filter((id) => id !== runId)
        : [...prev.selectedRuns, runId],
    }));
  };

  const toggleAllRuns = () => {
    setConfig((prev) => {
      const includeAllRuns = !prev.includeAllRuns;
      const selectedRuns = includeAllRuns
        ? prev.selectedAgents.flatMap((id) => agentRuns[id]?.map((r) => r.id) || [])
        : [];
      return { ...prev, includeAllRuns, selectedRuns };
    });
  };

  const startExperiment = async () => {
    if (config.selectedRuns.length === 0) {
      setError('Please select at least one run to evaluate');
      return;
    }

    setStep('running');
    setStartTime(new Date());
    setError(null);

    // Initialize progress map
    const progressMap = new Map<number, RunProgress>();
    config.selectedRuns.forEach((runId) => {
      progressMap.set(runId, { run_id: runId, status: 'pending' });
    });
    setRunProgress(progressMap);

    // Use SSE for real-time progress (fallback to bulk API if SSE not supported)
    const runIdsStr = config.selectedRuns.join(',');
    const sseUrl = `${import.meta.env.VITE_API_URL || 'http://localhost:8585'}/api/v1/benchmarks/evaluate-bulk/stream?run_ids=${runIdsStr}&concurrency=${config.concurrency}`;

    try {
      const eventSource = new EventSource(sseUrl);
      eventSourceRef.current = eventSource;

      eventSource.addEventListener('progress', (e) => {
        const progress: RunProgress = JSON.parse(e.data);
        setRunProgress((prev) => {
          const newMap = new Map(prev);
          newMap.set(progress.run_id, progress);
          return newMap;
        });
      });

      eventSource.addEventListener('complete', () => {
        eventSource.close();
        finishExperiment();
      });

      eventSource.onerror = (err) => {
        console.error('SSE error:', err);
        eventSource.close();
        // Fallback to bulk API
        fallbackToBulkAPI();
      };
    } catch (err) {
      console.error('Failed to start SSE:', err);
      fallbackToBulkAPI();
    }
  };

  const fallbackToBulkAPI = async () => {
    try {
      const response = await benchmarksApi.evaluateBulk({
        run_ids: config.selectedRuns,
        concurrency: config.concurrency,
      });

      const results = response.data;

      // Update progress with final results
      const progressMap = new Map<number, RunProgress>();
      results.results.forEach((result: RunProgress) => {
        progressMap.set(result.run_id, result);
      });
      setRunProgress(progressMap);

      finishExperiment();
    } catch (err) {
      console.error('Bulk evaluation failed:', err);
      setError('Evaluation failed. Please try again.');
      setStep('config');
    }
  };

  const finishExperiment = () => {
    const results = Array.from(runProgress.values());
    const completed = results.filter((r) => r.status === 'completed').length;
    const failed = results.filter((r) => r.status === 'failed').length;

    const completedResults = results.filter((r) => r.status === 'completed');
    const avgQualityScore = completedResults.length > 0
      ? completedResults.reduce((sum, r) => sum + (r.quality_score || 0), 0) / completedResults.length
      : 0;

    const productionReady = completedResults.filter((r) => r.production_ready).length;
    const productionReadyPct = completedResults.length > 0
      ? (productionReady / completedResults.length) * 100
      : 0;

    const finalResults: ExperimentResults = {
      totalRuns: config.selectedRuns.length,
      completed,
      failed,
      durationSeconds: elapsedTime,
      avgQualityScore,
      productionReadyPct,
      results,
    };

    setExperimentResults(finalResults);
    setStep('results');

    if (onComplete) {
      onComplete(finalResults);
    }
  };

  const reset = () => {
    setStep('config');
    setConfig({
      selectedAgents: [],
      selectedRuns: [],
      concurrency: 10,
      includeAllRuns: false,
    });
    setRunProgress(new Map());
    setExperimentResults(null);
    setStartTime(null);
    setElapsedTime(0);
    setError(null);

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }
  };

  const handleClose = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }
    if (timerRef.current) {
      clearInterval(timerRef.current);
    }
    reset();
    onClose();
  };

  if (!isOpen) return null;

  const completedCount = Array.from(runProgress.values()).filter((r) => r.status === 'completed').length;
  const failedCount = Array.from(runProgress.values()).filter((r) => r.status === 'failed').length;
  const evaluatingCount = Array.from(runProgress.values()).filter((r) => r.status === 'evaluating').length;
  const progressPct = config.selectedRuns.length > 0
    ? ((completedCount + failedCount) / config.selectedRuns.length) * 100
    : 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm">
      <div className="bg-white border border-gray-200 rounded-lg shadow-2xl w-full max-w-6xl max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 bg-white">
          <div>
            <h2 className="text-2xl font-semibold text-gray-900 flex items-center gap-2">
              <Zap className="h-6 w-6 text-purple-600" />
              Benchmark Experiment Runner
            </h2>
            <p className="text-sm text-gray-600 mt-1">
              {step === 'config' && 'Configure and run quality benchmarks across multiple agent runs'}
              {step === 'running' && 'Evaluating runs in parallel with LLM-as-judge metrics'}
              {step === 'results' && 'Experiment complete - Review quality analysis results'}
            </p>
          </div>
          <button
            onClick={handleClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-600" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          {error && (
            <div className="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-red-600 flex-shrink-0" />
              <span className="text-red-600 text-sm">{error}</span>
            </div>
          )}

          {/* Configuration Step */}
          {step === 'config' && (
            <div className="space-y-6">
              {/* Concurrency Setting */}
              <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg">
                <label className="block text-sm text-gray-900 mb-2">
                  Parallel Evaluations (Concurrency)
                </label>
                <div className="flex items-center gap-4">
                  <input
                    type="range"
                    min="1"
                    max="50"
                    value={config.concurrency}
                    onChange={(e) => setConfig({ ...config, concurrency: parseInt(e.target.value) })}
                    className="flex-1"
                  />
                  <span className="text-lg font-bold text-purple-600 w-16 text-right">
                    {config.concurrency}
                  </span>
                </div>
                <p className="text-xs text-gray-600 mt-2">
                  Higher concurrency = faster evaluation but more API costs
                </p>
              </div>

              {/* Agent Selection */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-lg font-semibold text-gray-900">
                    Select Agents ({config.selectedAgents.length} selected)
                  </h3>
                  <button
                    onClick={toggleAllRuns}
                    className={`px-3 py-1 text-xs rounded transition-colors ${
                      config.includeAllRuns
                        ? 'bg-green-600 text-white'
                        : 'bg-gray-100 text-gray-900 hover:bg-station-blue7'
                    }`}
                  >
                    {config.includeAllRuns ? 'All runs included' : 'Include all runs'}
                  </button>
                </div>

                {loading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader className="h-6 w-6 text-station-blue animate-spin" />
                  </div>
                ) : (
                  <div className="grid gap-3">
                    {agents.map((agent) => {
                      const runs = agentRuns[agent.id] || [];
                      const selectedRunsForAgent = runs.filter((r) =>
                        config.selectedRuns.includes(r.id)
                      ).length;
                      const isSelected = config.selectedAgents.includes(agent.id);

                      return (
                        <div
                          key={agent.id}
                          className={`p-4 border rounded-lg transition-all ${
                            isSelected
                              ? 'bg-station-blue/10 border-station-blue'
                              : 'bg-white border-gray-200 hover:border-gray-300'
                          }`}
                        >
                          <div className="flex items-start gap-3">
                            <input
                              type="checkbox"
                              checked={isSelected}
                              onChange={() => toggleAgent(agent.id)}
                              className="mt-1"
                            />
                            <div className="flex-1">
                              <div className="flex items-center justify-between">
                                <h4 className="font-semibold text-gray-900">
                                  {agent.name}
                                </h4>
                                <span className="text-xs text-gray-600">
                                  {runs.length} runs
                                  {config.includeAllRuns && isSelected && (
                                    <span className="text-green-600 ml-2">• All included</span>
                                  )}
                                  {!config.includeAllRuns && selectedRunsForAgent > 0 && (
                                    <span className="text-station-blue ml-2">
                                      • {selectedRunsForAgent} selected
                                    </span>
                                  )}
                                </span>
                              </div>
                              {agent.description && (
                                <p className="text-sm text-gray-600 mt-1">
                                  {agent.description}
                                </p>
                              )}

                              {/* Run selection (only show if agent selected and not includeAllRuns) */}
                              {isSelected && !config.includeAllRuns && runs.length > 0 && (
                                <div className="mt-3 pl-4 border-l-2 border-gray-300 space-y-2">
                                  {runs.slice(0, 10).map((run) => (
                                    <label
                                      key={run.id}
                                      className="flex items-start gap-2 text-sm cursor-pointer hover:bg-gray-100/50 p-2 rounded"
                                    >
                                      <input
                                        type="checkbox"
                                        checked={config.selectedRuns.includes(run.id)}
                                        onChange={() => toggleRun(run.id)}
                                        className="mt-0.5"
                                      />
                                      <div className="flex-1">
                                        <div className="text-gray-600">
                                          Run #{run.id} • {run.status}
                                        </div>
                                        <div className="text-xs text-gray-600 truncate">
                                          {run.task.substring(0, 100)}...
                                        </div>
                                      </div>
                                    </label>
                                  ))}
                                  {runs.length > 10 && (
                                    <div className="text-xs text-gray-600 italic pl-2">
                                      + {runs.length - 10} more runs
                                    </div>
                                  )}
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Summary */}
              {config.selectedRuns.length > 0 && (
                <div className="p-4 bg-purple-600/10 border border-purple-600 rounded-lg">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-lg font-semibold text-purple-600">
                        Ready to evaluate {config.selectedRuns.length} runs
                      </div>
                      <div className="text-sm text-gray-600 mt-1">
                        Estimated cost: ~${(config.selectedRuns.length * 0.0004).toFixed(4)} •
                        Duration: ~{Math.ceil((config.selectedRuns.length / config.concurrency) * 4)}s
                      </div>
                    </div>
                    <button
                      onClick={startExperiment}
                      className="flex items-center gap-2 px-6 py-3 bg-purple-600 text-white hover:bg-opacity-90 rounded font-semibold transition-colors"
                    >
                      <Play className="h-5 w-5" />
                      Start Experiment
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Running Step */}
          {step === 'running' && (
            <div className="space-y-6">
              {/* Progress Overview */}
              <div className="grid grid-cols-4 gap-4">
                <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg">
                  <div className="text-sm text-gray-600 mb-1">Total Runs</div>
                  <div className="text-2xl font-bold text-gray-900">
                    {config.selectedRuns.length}
                  </div>
                </div>
                <div className="p-4 bg-green-600/10 border border-green-600 rounded-lg">
                  <div className="text-sm text-gray-600 mb-1">Completed</div>
                  <div className="text-2xl font-bold text-green-600">
                    {completedCount}
                  </div>
                </div>
                <div className="p-4 bg-station-blue/10 border border-station-blue rounded-lg">
                  <div className="text-sm text-gray-600 mb-1">Evaluating</div>
                  <div className="text-2xl font-bold text-station-blue">
                    {evaluatingCount}
                  </div>
                </div>
                <div className="p-4 bg-red-600/10 border border-red-600 rounded-lg">
                  <div className="text-sm text-gray-600 mb-1">Failed</div>
                  <div className="text-2xl font-bold text-red-600">
                    {failedCount}
                  </div>
                </div>
              </div>

              {/* Progress Bar */}
              <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg">
                <div className="flex items-center justify-between mb-2">
                  <div className="text-sm text-gray-600">
                    Progress: {Math.floor(progressPct)}%
                  </div>
                  <div className="text-sm text-gray-600 flex items-center gap-2">
                    <Clock className="h-4 w-4" />
                    {elapsedTime}s elapsed
                  </div>
                </div>
                <div className="w-full bg-gray-100 rounded-full h-3 overflow-hidden">
                  <div
                    className="bg-gradient-to-r from-station-blue to-purple-600 h-full transition-all duration-300"
                    style={{ width: `${progressPct}%` }}
                  />
                </div>
              </div>

              {/* Individual Run Progress */}
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {Array.from(runProgress.values()).map((progress) => (
                  <div
                    key={progress.run_id}
                    className={`p-3 border rounded-lg flex items-center justify-between ${
                      progress.status === 'completed'
                        ? 'bg-green-600/10 border-green-600'
                        : progress.status === 'failed'
                        ? 'bg-red-600/10 border-red-600'
                        : progress.status === 'evaluating'
                        ? 'bg-station-blue/10 border-station-blue'
                        : 'bg-white border-gray-200'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      {progress.status === 'completed' && (
                        <CheckCircle className="h-5 w-5 text-green-600" />
                      )}
                      {progress.status === 'failed' && (
                        <XCircle className="h-5 w-5 text-red-600" />
                      )}
                      {progress.status === 'evaluating' && (
                        <Loader className="h-5 w-5 text-station-blue animate-spin" />
                      )}
                      {progress.status === 'pending' && (
                        <div className="h-5 w-5 border-2 border-gray-200 rounded-full" />
                      )}
                      <div>
                        <div className="text-sm text-gray-900">
                          Run #{progress.run_id}
                        </div>
                        {progress.error && (
                          <div className="text-xs text-red-600">{progress.error}</div>
                        )}
                      </div>
                    </div>

                    {progress.status === 'completed' && (
                      <div className="flex items-center gap-4">
                        <div className="text-right">
                          <div className="text-xs text-gray-600">Quality Score</div>
                          <div
                            className={`text-lg font-bold ${
                              (progress.quality_score || 0) >= 8.5
                                ? 'text-green-600'
                                : (progress.quality_score || 0) >= 7
                                ? 'text-yellow-600'
                                : 'text-red-600'
                            }`}
                          >
                            {progress.quality_score?.toFixed(1)}/10
                          </div>
                        </div>
                        {progress.production_ready && (
                          <div className="px-2 py-1 bg-green-600 text-white text-xs rounded">
                            PROD READY
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Results Step */}
          {step === 'results' && experimentResults && (
            <div className="space-y-6">
              {/* Summary Cards */}
              <div className="grid grid-cols-2 gap-4">
                <div className="p-6 bg-gradient-to-br from-purple-100 to-blue-100 border border-purple-600 rounded-lg">
                  <div className="flex items-center gap-3 mb-2">
                    <TrendingUp className="h-6 w-6 text-purple-600" />
                    <div className="text-sm text-gray-600">
                      Average Quality Score
                    </div>
                  </div>
                  <div
                    className={`text-4xl font-bold ${
                      experimentResults.avgQualityScore >= 8.5
                        ? 'text-green-600'
                        : experimentResults.avgQualityScore >= 7
                        ? 'text-yellow-600'
                        : 'text-red-600'
                    }`}
                  >
                    {experimentResults.avgQualityScore.toFixed(1)}/10
                  </div>
                </div>

                <div className="p-6 bg-gradient-to-br from-green-100 to-blue-100 border border-green-600 rounded-lg">
                  <div className="flex items-center gap-3 mb-2">
                    <CheckCircle className="h-6 w-6 text-green-600" />
                    <div className="text-sm text-gray-600">
                      Production Ready
                    </div>
                  </div>
                  <div className="text-4xl font-bold text-green-600">
                    {Math.floor(experimentResults.productionReadyPct)}%
                  </div>
                  <div className="text-sm text-gray-600 mt-1">
                    {experimentResults.results.filter((r) => r.production_ready).length} of{' '}
                    {experimentResults.completed} runs
                  </div>
                </div>
              </div>

              {/* Experiment Stats */}
              <div className="grid grid-cols-4 gap-3">
                <div className="p-4 bg-white border border-gray-200 rounded">
                  <div className="text-xs text-gray-600 mb-1">Total Runs</div>
                  <div className="text-xl font-bold text-gray-900">
                    {experimentResults.totalRuns}
                  </div>
                </div>
                <div className="p-4 bg-white border border-gray-200 rounded">
                  <div className="text-xs text-gray-600 mb-1">Completed</div>
                  <div className="text-xl font-bold text-green-600">
                    {experimentResults.completed}
                  </div>
                </div>
                <div className="p-4 bg-white border border-gray-200 rounded">
                  <div className="text-xs text-gray-600 mb-1">Failed</div>
                  <div className="text-xl font-bold text-red-600">
                    {experimentResults.failed}
                  </div>
                </div>
                <div className="p-4 bg-white border border-gray-200 rounded">
                  <div className="text-xs text-gray-600 mb-1 flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    Duration
                  </div>
                  <div className="text-xl font-bold text-purple-600">
                    {experimentResults.durationSeconds.toFixed(1)}s
                  </div>
                </div>
              </div>

              {/* Quality Distribution */}
              <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg">
                <h3 className="text-lg font-semibold text-gray-900 mb-3">
                  Quality Score Distribution
                </h3>
                <div className="space-y-2">
                  {[
                    { range: '9.0-10.0', label: 'Excellent', color: 'green-600' },
                    { range: '8.0-8.9', label: 'Good', color: 'blue-600' },
                    { range: '7.0-7.9', label: 'Fair', color: 'yellow-600' },
                    { range: '0.0-6.9', label: 'Needs Work', color: 'red-600' },
                  ].map((bucket) => {
                    const count = experimentResults.results.filter((r) => {
                      if (!r.quality_score) return false;
                      const [min, max] = bucket.range.split('-').map(parseFloat);
                      return r.quality_score >= min && r.quality_score <= max;
                    }).length;
                    const pct =
                      experimentResults.completed > 0
                        ? (count / experimentResults.completed) * 100
                        : 0;

                    return (
                      <div key={bucket.range}>
                        <div className="flex items-center justify-between mb-1">
                          <div className="text-sm text-gray-900">
                            {bucket.range} - {bucket.label}
                          </div>
                          <div className="text-sm text-gray-600">
                            {count} runs ({Math.floor(pct)}%)
                          </div>
                        </div>
                        <div className="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
                          <div
                            className={`bg-${bucket.color} h-full transition-all duration-300`}
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-200 bg-white flex items-center justify-between">
          <div className="text-sm text-gray-600">
            {step === 'config' && `${config.selectedRuns.length} runs selected`}
            {step === 'running' && `Evaluating ${config.selectedRuns.length} runs...`}
            {step === 'results' && `Experiment completed in ${experimentResults?.durationSeconds.toFixed(1)}s`}
          </div>
          <div className="flex items-center gap-2">
            {step === 'results' && (
              <>
                <button
                  onClick={reset}
                  className="px-4 py-2 bg-gray-100 text-gray-900 hover:bg-station-blue7 rounded text-sm transition-colors"
                >
                  New Experiment
                </button>
                <button
                  onClick={handleClose}
                  className="px-4 py-2 bg-station-blue text-white hover:bg-opacity-90 rounded text-sm transition-colors"
                >
                  Done
                </button>
              </>
            )}
            {step === 'config' && (
              <button
                onClick={handleClose}
                className="px-4 py-2 bg-gray-100 text-gray-900 hover:bg-station-blue7 rounded text-sm transition-colors"
              >
                Cancel
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
