import React, { useState, useEffect } from 'react';
import { Loader, TrendingUp, CheckCircle, XCircle, AlertTriangle, Sparkles } from 'lucide-react';
import { benchmarksApi } from '../../api/station';
import type { BenchmarkMetric, BenchmarkResult } from '../../types/station';

interface BenchmarkTabProps {
  runId: number;
}

export const BenchmarkTab: React.FC<BenchmarkTabProps> = ({ runId }) => {
  const [metrics, setMetrics] = useState<BenchmarkMetric[]>([]);
  const [loading, setLoading] = useState(false);
  const [evaluating, setEvaluating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<BenchmarkResult | null>(null);

  useEffect(() => {
    fetchMetrics();
  }, [runId]);

  const fetchMetrics = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await benchmarksApi.getMetrics(runId);
      setMetrics(response.data.metrics || []);
      
      // Calculate aggregate scores if metrics exist
      if (response.data.metrics && response.data.metrics.length > 0) {
        // Calculate quality score properly: average of normalized metrics (already 0-10 scale)
        let totalScore = 0;
        response.data.metrics.forEach(m => {
          // Invert scores for metrics where lower is better (hallucination, toxicity)
          const isLowerBetter = m.metric_name === 'hallucination' || 
                                m.metric_name === 'toxicity' || 
                                m.metric_name === 'bias';
          const normalizedScore = isLowerBetter ? (1.0 - m.score) * 10.0 : m.score * 10.0;
          totalScore += normalizedScore;
        });
        const qualityScore = totalScore / response.data.metrics.length;
        const passCount = response.data.metrics.filter(m => m.passed).length;
        const productionReady = passCount === response.data.metrics.length;
        
        // Create a mock result for display
        const metricsMap: Record<string, any> = {};
        response.data.metrics.forEach(m => {
          metricsMap[m.metric_name] = {
            metric_type: m.metric_name,
            score: m.score,
            threshold: m.threshold,
            passed: m.passed,
            reason: m.reason,
            judge_tokens: 0,
            judge_cost: 0,
            evaluation_duration_ms: 0,
          };
        });
        
        setResult({
          run_id: runId,
          agent_id: 0,
          task: '',
          quality_score: qualityScore,
          production_ready: productionReady,
          recommendation: productionReady 
            ? 'This run meets all quality thresholds and is ready for production use.' 
            : 'This run has quality issues that should be addressed before production deployment.',
          metrics: metricsMap,
          total_judge_tokens: 0,
          total_judge_cost: 0,
          evaluation_time_ms: 0,
        });
      }
    } catch (err: any) {
      if (err.response?.status === 404) {
        // No metrics found - this is normal for unevaluated runs
        setMetrics([]);
        setResult(null);
      } else {
        console.error('Failed to fetch benchmark metrics:', err);
        setError(err.response?.data?.error || 'Failed to load benchmark metrics');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleEvaluate = async () => {
    setEvaluating(true);
    setError(null);
    
    try {
      const response = await benchmarksApi.evaluate(runId);
      setResult(response.data);
      
      // Refresh metrics after evaluation
      await fetchMetrics();
    } catch (err: any) {
      console.error('Failed to evaluate run:', err);
      setError(err.response?.data?.error || err.response?.data?.details || 'Failed to evaluate run');
    } finally {
      setEvaluating(false);
    }
  };

  const getScoreColor = (score: number): string => {
    if (score >= 8.5) return 'text-green-700';
    if (score >= 7.0) return 'text-yellow-700';
    return 'text-red-700';
  };

  const getScoreBg = (score: number): string => {
    if (score >= 8.5) return 'bg-green-50 border-green-200';
    if (score >= 7.0) return 'bg-yellow-50 border-yellow-200';
    return 'bg-red-50 border-red-200';
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader className="h-8 w-8 text-gray-400 animate-spin" />
        <span className="ml-3 text-gray-600">Loading benchmark data...</span>
      </div>
    );
  }

  if (error && metrics.length === 0) {
    return (
      <div className="text-center py-12">
        <div className="text-red-600 mb-2">{error}</div>
      </div>
    );
  }

  if (metrics.length === 0 && !result) {
    return (
      <div className="text-center py-12">
        <Sparkles className="h-12 w-12 text-purple-500 mx-auto mb-4" />
        <h3 className="text-lg font-semibold text-gray-900 mb-2">Quality Evaluation</h3>
        <p className="text-gray-600 mb-6 max-w-md mx-auto">
          Evaluate this run using LLM-as-judge metrics to assess quality, hallucination, relevancy, and production readiness.
        </p>
        <button
          onClick={handleEvaluate}
          disabled={evaluating}
          className="px-6 py-3 bg-purple-600 hover:bg-purple-700 disabled:bg-gray-400 disabled:cursor-not-allowed text-white rounded-lg transition-colors flex items-center gap-2 mx-auto shadow-sm"
        >
          {evaluating ? (
            <>
              <Loader className="h-5 w-5 animate-spin" />
              Evaluating...
            </>
          ) : (
            <>
              <TrendingUp className="h-5 w-5" />
              Evaluate Run Quality
            </>
          )}
        </button>
        {error && (
          <div className="mt-4 text-red-600 text-sm">{error}</div>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Overall Score Card */}
      {result && (
        <div className={`border-2 rounded-lg p-6 shadow-sm ${getScoreBg(result.quality_score)}`}>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 mb-1">Quality Score</h3>
              <p className="text-gray-600 text-sm">Overall evaluation across all metrics</p>
            </div>
            <div className={`text-5xl font-bold ${getScoreColor(result.quality_score)}`}>
              {result.quality_score.toFixed(1)}<span className="text-2xl">/10</span>
            </div>
          </div>
          
          <div className="flex items-center gap-2 mb-3">
            {result.production_ready ? (
              <>
                <CheckCircle className="h-5 w-5 text-green-600" />
                <span className="text-green-700 font-semibold">Production Ready</span>
              </>
            ) : (
              <>
                <AlertTriangle className="h-5 w-5 text-yellow-600" />
                <span className="text-yellow-700 font-semibold">Needs Improvement</span>
              </>
            )}
          </div>
          
          <p className="text-gray-700 text-sm">{result.recommendation}</p>
          
          {result.total_judge_cost > 0 && (
            <div className="mt-4 pt-4 border-t border-gray-200 text-xs text-gray-600">
              Evaluation cost: ${result.total_judge_cost.toFixed(4)} • {result.total_judge_tokens.toLocaleString()} tokens • {result.evaluation_time_ms}ms
            </div>
          )}
        </div>
      )}

      {/* Metrics Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {metrics.map((metric) => (
          <div
            key={metric.id}
            className={`border-2 rounded-lg p-4 shadow-sm ${
              metric.passed
                ? 'bg-green-50 border-green-200'
                : 'bg-red-50 border-red-200'
            }`}
          >
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                {metric.passed ? (
                  <CheckCircle className="h-5 w-5 text-green-600" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-600" />
                )}
                <h4 className="font-semibold text-gray-900 capitalize">
                  {metric.metric_name.replace(/_/g, ' ')}
                </h4>
              </div>
              <span className={`text-2xl font-bold ${
                metric.passed ? 'text-green-600' : 'text-red-600'
              }`}>
                {metric.score.toFixed(2)}
              </span>
            </div>
            
            <div className="mb-3">
              <div className="flex items-center justify-between text-xs text-gray-600 mb-1">
                <span>Threshold: {metric.threshold.toFixed(2)}</span>
                <span className="font-semibold">{metric.passed ? 'PASS' : 'FAIL'}</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className={`h-2 rounded-full transition-all ${
                    metric.passed ? 'bg-green-500' : 'bg-red-500'
                  }`}
                  style={{ width: `${Math.min(metric.score * 100, 100)}%` }}
                />
              </div>
            </div>
            
            {metric.reason && (
              <p className="text-xs text-gray-600 leading-relaxed">
                {metric.reason}
              </p>
            )}
          </div>
        ))}
      </div>

      {/* Re-evaluate button */}
      {metrics.length > 0 && (
        <div className="text-center pt-4">
          <button
            onClick={handleEvaluate}
            disabled={evaluating}
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:cursor-not-allowed text-gray-200 font-mono rounded transition-colors text-sm"
          >
            {evaluating ? 'Re-evaluating...' : 'Re-evaluate Run'}
          </button>
        </div>
      )}
    </div>
  );
};
