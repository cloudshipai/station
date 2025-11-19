import React from 'react';
import { Bot, Search } from 'lucide-react';

interface AgentsSidebarProps {
  agents: any[];
  selectedAgentId?: number;
  onSelectAgent: (agent: any) => void;
}

export const AgentsSidebar: React.FC<AgentsSidebarProps> = ({ agents, selectedAgentId, onSelectAgent }) => {
  const [searchQuery, setSearchQuery] = React.useState('');
  
  const filteredAgents = agents.filter(agent => 
    agent.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    agent.description?.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="h-full flex flex-col bg-white">
      {/* Search Header */}
      <div className="p-4 border-b border-gray-200/60">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
          <input 
            type="text" 
            placeholder="Search agents..." 
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-9 pr-3 py-2.5 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-gray-900/10 focus:border-gray-300 transition-all placeholder:text-gray-400"
          />
        </div>
      </div>
      
      {/* Agent List */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {filteredAgents.length === 0 ? (
          <div className="text-center py-12 px-4">
            <Bot className="h-12 w-12 text-gray-300 mx-auto mb-3" />
            <p className="text-sm text-gray-500">No agents found</p>
            {searchQuery && (
              <button 
                onClick={() => setSearchQuery('')}
                className="text-xs text-gray-600 hover:text-gray-900 mt-2 underline"
              >
                Clear search
              </button>
            )}
          </div>
        ) : (
          filteredAgents.map((agent) => (
            <div 
              key={agent.id} 
              onClick={() => onSelectAgent(agent)}
              className={`p-3.5 rounded-xl border cursor-pointer transition-all group ${
                selectedAgentId === agent.id 
                  ? 'bg-gray-900 border-gray-900 shadow-lg' 
                  : 'bg-white border-gray-200/60 hover:border-gray-300 hover:shadow-md'
              }`}
            >
              <div className="flex items-start gap-3">
                <div className={`p-2 rounded-lg transition-all ${
                  selectedAgentId === agent.id 
                    ? 'bg-white/20 text-white' 
                    : 'bg-gray-100 text-gray-600 group-hover:bg-gray-200'
                }`}>
                  <Bot className="h-4 w-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className={`text-sm font-medium truncate ${
                    selectedAgentId === agent.id ? 'text-white' : 'text-gray-900'
                  }`}>
                    {agent.name}
                  </div>
                  <div className={`text-xs mt-1.5 flex items-center gap-1.5 ${
                    selectedAgentId === agent.id ? 'text-gray-300' : 'text-gray-500'
                  }`}>
                    <span className="truncate font-mono">{agent.model || 'gpt-4o-mini'}</span>
                  </div>
                </div>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
};
