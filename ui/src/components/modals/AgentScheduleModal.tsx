import React, { useState, useEffect } from 'react';
import { X, Clock, Calendar } from 'lucide-react';
import { apiClient } from '../../api/client';

interface AgentScheduleModalProps {
  isOpen: boolean;
  onClose: () => void;
  agentId: number;
  agentName: string;
  currentSchedule?: string;
  currentEnabled: boolean;
  currentScheduleVariables?: string;
  onSuccess?: () => void;
}

export const AgentScheduleModal: React.FC<AgentScheduleModalProps> = ({
  isOpen,
  onClose,
  agentId,
  agentName,
  currentSchedule,
  currentEnabled,
  currentScheduleVariables,
  onSuccess,
}) => {
  const [cronSchedule, setCronSchedule] = useState(currentSchedule || '');
  const [scheduleEnabled, setScheduleEnabled] = useState(currentEnabled);
  const [scheduleVariables, setScheduleVariables] = useState(currentScheduleVariables || '{}');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // Common cron presets (with seconds field for 6-field format)
  const presets = [
    { label: 'Every minute', value: '0 * * * * *' },
    { label: 'Every 5 minutes', value: '0 */5 * * * *' },
    { label: 'Every 15 minutes', value: '0 */15 * * * *' },
    { label: 'Every 30 minutes', value: '0 */30 * * * *' },
    { label: 'Every hour', value: '0 0 * * * *' },
    { label: 'Every 6 hours', value: '0 0 */6 * * *' },
    { label: 'Every day at midnight', value: '0 0 0 * * *' },
    { label: 'Every day at 9 AM', value: '0 0 9 * * *' },
    { label: 'Every Monday at 9 AM', value: '0 0 9 * * 1' },
    { label: 'Every weekday at 9 AM', value: '0 0 9 * * 1-5' },
  ];

  useEffect(() => {
    if (isOpen) {
      setCronSchedule(currentSchedule || '');
      setScheduleEnabled(currentEnabled);
      setScheduleVariables(currentScheduleVariables || '{}');
      setError('');
    }
  }, [isOpen, currentSchedule, currentEnabled, currentScheduleVariables]);

  const handleSave = async () => {
    if (scheduleEnabled && !cronSchedule.trim()) {
      setError('Please enter a cron schedule or disable scheduling');
      return;
    }

    // Validate JSON format for schedule_variables
    if (scheduleEnabled && scheduleVariables.trim()) {
      try {
        JSON.parse(scheduleVariables);
      } catch (e) {
        setError('Schedule variables must be valid JSON');
        return;
      }
    }

    setSaving(true);
    setError('');

    try {
      await apiClient.put(`/admin/agents/${agentId}`, {
        cron_schedule: cronSchedule.trim() || null,
        schedule_enabled: scheduleEnabled,
        schedule_variables: scheduleVariables.trim() || null,
      });

      if (onSuccess) {
        onSuccess();
      }
      onClose();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to update schedule');
    } finally {
      setSaving(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Clock className="h-6 w-6 text-tokyo-green" />
            <div>
              <h2 className="text-xl font-mono font-semibold text-tokyo-fg">
                Schedule Agent
              </h2>
              <p className="text-sm text-tokyo-comment font-mono">{agentName}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        {/* Enable/Disable Toggle */}
        <div className="mb-6 p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={scheduleEnabled}
              onChange={(e) => setScheduleEnabled(e.target.checked)}
              className="w-5 h-5 text-tokyo-green bg-tokyo-bg border-tokyo-blue7 rounded focus:ring-tokyo-green"
            />
            <div>
              <div className="font-mono text-tokyo-fg">Enable Scheduled Execution</div>
              <div className="text-xs text-tokyo-comment font-mono">
                Run this agent automatically on a schedule
              </div>
            </div>
          </label>
        </div>

        {/* Cron Schedule Input */}
        {scheduleEnabled && (
          <>
            <div className="mb-4">
              <label className="block text-sm font-mono text-tokyo-fg mb-2">
                Cron Schedule Expression
              </label>
              <input
                type="text"
                value={cronSchedule}
                onChange={(e) => setCronSchedule(e.target.value)}
                placeholder="0 0 * * *"
                className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono rounded focus:outline-none focus:border-tokyo-blue"
              />
              <div className="mt-1 text-xs text-tokyo-comment font-mono">
                Format: second minute hour day month weekday (e.g., "0 0 0 * * *" = daily at midnight)
              </div>
            </div>

            {/* Presets */}
            <div className="mb-6">
              <label className="block text-sm font-mono text-tokyo-fg mb-2">
                Quick Presets
              </label>
              <div className="grid grid-cols-2 gap-2">
                {presets.map((preset) => (
                  <button
                    key={preset.value}
                    onClick={() => setCronSchedule(preset.value)}
                    className={`p-2 text-left border rounded font-mono text-xs transition-colors ${
                      cronSchedule === preset.value
                        ? 'bg-tokyo-blue/20 border-tokyo-blue text-tokyo-blue'
                        : 'bg-tokyo-bg border-tokyo-blue7 text-tokyo-fg hover:border-tokyo-blue5'
                    }`}
                  >
                    <div className="font-semibold">{preset.label}</div>
                    <div className="text-tokyo-comment">{preset.value}</div>
                  </button>
                ))}
              </div>
            </div>

            {/* Cron Reference */}
            <div className="mb-6 p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
              <div className="flex items-center gap-2 mb-2">
                <Calendar className="h-4 w-4 text-tokyo-cyan" />
                <span className="font-mono text-sm text-tokyo-fg">Cron Format Reference</span>
              </div>
              <div className="space-y-1 text-xs font-mono text-tokyo-comment">
                <div><span className="text-tokyo-green">*</span> = any value</div>
                <div><span className="text-tokyo-green">*/5</span> = every 5 units</div>
                <div><span className="text-tokyo-green">0-5</span> = range (0 through 5)</div>
                <div><span className="text-tokyo-green">1,3,5</span> = list (1, 3, and 5)</div>
              </div>
            </div>

            {/* Schedule Variables */}
            <div className="mb-6">
              <label className="block text-sm font-mono text-tokyo-fg mb-2">
                Schedule Variables (JSON)
              </label>
              <textarea
                value={scheduleVariables}
                onChange={(e) => setScheduleVariables(e.target.value)}
                placeholder='{"project_path": "/workspace", "scan_type": "full"}'
                rows={4}
                className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono text-sm rounded focus:outline-none focus:border-tokyo-blue"
              />
              <div className="mt-1 text-xs text-tokyo-comment font-mono">
                Provide JSON object with variables to pass to the agent on each scheduled run. These will be merged with the agent's input schema.
              </div>
            </div>

            {/* Execution Info */}
            <div className="mb-6 p-4 bg-tokyo-blue/10 border border-tokyo-blue7 rounded">
              <div className="font-mono text-sm text-tokyo-fg mb-2">ℹ️ Scheduled Execution</div>
              <div className="text-xs text-tokyo-comment font-mono space-y-1">
                <div>• The agent will execute with its system prompt as the task</div>
                <div>• Use Schedule Variables to provide input values for agents with input schemas</div>
                <div>• Check the Runs page to monitor scheduled executions</div>
              </div>
            </div>
          </>
        )}

        {/* Error Message */}
        {error && (
          <div className="mb-4 p-3 bg-tokyo-red/20 border border-tokyo-red rounded">
            <p className="text-sm text-tokyo-red font-mono">{error}</p>
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono rounded hover:bg-tokyo-dark2 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2 bg-tokyo-green text-tokyo-bg font-mono rounded hover:bg-tokyo-green/80 transition-colors disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save Schedule'}
          </button>
        </div>
      </div>
    </div>
  );
};
