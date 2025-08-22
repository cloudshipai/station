import React, { useMemo, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell, LineChart, Line } from 'recharts';

interface Run {
  id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed';
  duration_seconds?: number;
  started_at: string;
  total_tokens?: number;
  input_tokens?: number;
  output_tokens?: number;
  tools_used?: number;
  steps_taken?: number;
}

interface StatsTabProps {
  runs: Run[];
}

export const StatsTab: React.FC<StatsTabProps> = ({ runs }) => {
  const [filterAgent, setFilterAgent] = useState<string>('all');

  const completedRuns = runs.filter(run => run.status === 'completed');
  const filteredRuns = filterAgent === 'all' 
    ? completedRuns 
    : completedRuns.filter(run => run.agent_name === filterAgent);

  const agentNames = Array.from(new Set(completedRuns.map(run => run.agent_name)));

  const stats = useMemo(() => {
    // Agent performance stats
    const agentStats = agentNames.map(agentName => {
      const agentRuns = completedRuns.filter(run => run.agent_name === agentName);
      const totalRuns = agentRuns.length;
      const avgDuration = agentRuns.reduce((sum, run) => sum + (run.duration_seconds || 0), 0) / totalRuns;
      const totalTokens = agentRuns.reduce((sum, run) => sum + (run.total_tokens || 0), 0);
      const avgTokens = totalTokens / totalRuns;

      return {
        agent: agentName,
        runs: totalRuns,
        avgDuration: Math.round(avgDuration * 10) / 10,
        totalTokens,
        avgTokens: Math.round(avgTokens)
      };
    });

    // Status distribution
    const statusStats = [
      { name: 'Completed', value: runs.filter(r => r.status === 'completed').length, color: '#9ece6a' },
      { name: 'Running', value: runs.filter(r => r.status === 'running').length, color: '#7aa2f7' },
      { name: 'Failed', value: runs.filter(r => r.status === 'failed').length, color: '#f7768e' }
    ].filter(s => s.value > 0);

    // Duration over time (last 30 runs)
    const recentRuns = filteredRuns.slice(-30).map((run, index) => ({
      run: index + 1,
      duration: run.duration_seconds || 0,
      tokens: run.total_tokens || 0,
      date: new Date(run.started_at).toLocaleDateString()
    }));

    // Token usage stats
    const tokenStats = filteredRuns.map(run => ({
      run_id: run.id,
      input_tokens: run.input_tokens || 0,
      output_tokens: run.output_tokens || 0,
      total_tokens: run.total_tokens || 0
    })).slice(-20); // Last 20 runs

    return { agentStats, statusStats, recentRuns, tokenStats };
  }, [runs, completedRuns, filteredRuns, agentNames]);

  const COLORS = ['#9ece6a', '#7aa2f7', '#f7768e', '#e0af68', '#bb9af7'];

  return (
    <div className="space-y-6">
      {/* Filter Controls */}
      <div className="flex items-center gap-4">
        <label className="text-sm font-mono text-tokyo-comment">Filter by Agent:</label>
        <select
          value={filterAgent}
          onChange={(e) => setFilterAgent(e.target.value)}
          className="px-3 py-2 bg-tokyo-bg-dark border border-tokyo-blue7 rounded text-tokyo-fg font-mono text-sm focus:outline-none focus:border-tokyo-blue"
        >
          <option value="all">All Agents</option>
          {agentNames.map(name => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
      </div>

      {/* Overview Stats */}
      <div className="grid grid-cols-4 gap-4">
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <div className="text-2xl font-mono font-bold text-tokyo-green">{filteredRuns.length}</div>
          <div className="text-sm text-tokyo-comment">Total Runs</div>
        </div>
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <div className="text-2xl font-mono font-bold text-tokyo-blue">
            {filteredRuns.reduce((sum, run) => sum + (run.total_tokens || 0), 0).toLocaleString()}
          </div>
          <div className="text-sm text-tokyo-comment">Total Tokens</div>
        </div>
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <div className="text-2xl font-mono font-bold text-tokyo-purple">
            {filteredRuns.length > 0 ? Math.round(filteredRuns.reduce((sum, run) => sum + (run.duration_seconds || 0), 0) / filteredRuns.length * 10) / 10 : 0}s
          </div>
          <div className="text-sm text-tokyo-comment">Avg Duration</div>
        </div>
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <div className="text-2xl font-mono font-bold text-tokyo-cyan">
            {filteredRuns.reduce((sum, run) => sum + (run.tools_used || 0), 0)}
          </div>
          <div className="text-sm text-tokyo-comment">Tools Used</div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* Agent Performance Chart */}
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <h3 className="text-lg font-mono font-medium text-tokyo-green mb-4">Agent Performance</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={stats.agentStats}>
              <CartesianGrid strokeDasharray="3 3" stroke="#414868" />
              <XAxis dataKey="agent" stroke="#565f89" />
              <YAxis stroke="#565f89" />
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: '#24283b', 
                  border: '1px solid #565f89',
                  borderRadius: '8px',
                  color: '#c0caf5',
                  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.8)'
                }} 
              />
              <Legend />
              <Bar dataKey="runs" fill="#7aa2f7" name="Total Runs" />
              <Bar dataKey="avgDuration" fill="#9ece6a" name="Avg Duration (s)" />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Status Distribution */}
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <h3 className="text-lg font-mono font-medium text-tokyo-green mb-4">Run Status Distribution</h3>
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie
                data={stats.statusStats}
                cx="50%"
                cy="50%"
                labelLine={false}
                label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                outerRadius={100}
                fill="#8884d8"
                dataKey="value"
              >
                {stats.statusStats.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: '#24283b', 
                  border: '1px solid #565f89',
                  borderRadius: '8px',
                  color: '#c0caf5',
                  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.8)'
                }} 
              />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* Duration Trends */}
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <h3 className="text-lg font-mono font-medium text-tokyo-green mb-4">Duration Trends (Last 30 Runs)</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={stats.recentRuns}>
              <CartesianGrid strokeDasharray="3 3" stroke="#414868" />
              <XAxis dataKey="run" stroke="#565f89" />
              <YAxis stroke="#565f89" />
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: '#24283b', 
                  border: '1px solid #565f89',
                  borderRadius: '8px',
                  color: '#c0caf5',
                  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.8)'
                }} 
              />
              <Line type="monotone" dataKey="duration" stroke="#bb9af7" strokeWidth={2} dot={{ fill: '#bb9af7' }} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* Token Usage */}
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
          <h3 className="text-lg font-mono font-medium text-tokyo-green mb-4">Token Usage (Last 20 Runs)</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={stats.tokenStats}>
              <CartesianGrid strokeDasharray="3 3" stroke="#414868" />
              <XAxis dataKey="run_id" stroke="#565f89" />
              <YAxis stroke="#565f89" />
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: '#24283b', 
                  border: '1px solid #565f89',
                  borderRadius: '8px',
                  color: '#c0caf5',
                  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.8)'
                }} 
              />
              <Legend />
              <Bar dataKey="input_tokens" stackId="a" fill="#7dcfff" name="Input Tokens" />
              <Bar dataKey="output_tokens" stackId="a" fill="#e0af68" name="Output Tokens" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
};