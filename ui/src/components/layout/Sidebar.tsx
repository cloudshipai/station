import React from 'react';
import { Bot, Server, Library, Play, Layers, Package, Settings, Cloud } from 'lucide-react';

interface SidebarItem {
  id: string;
  label: string;
  icon: React.ComponentType<any>;
  path: string;
}

interface SidebarProps {
  currentPage: string;
  onPageChange: (page: string) => void;
}

const sidebarItems: SidebarItem[] = [
  { id: 'agents', label: 'Agents', icon: Bot, path: '/agents' },
  { id: 'mcps', label: 'MCP Servers', icon: Server, path: '/mcp-servers' },
  { id: 'mcp-directory', label: 'MCP Directory', icon: Library, path: '/mcp-directory' },
  { id: 'runs', label: 'Runs', icon: Play, path: '/runs' },
  { id: 'environments', label: 'Environments', icon: Layers, path: '/environments' },
  { id: 'bundles', label: 'Bundles', icon: Package, path: '/bundles' },
  { id: 'cloudship', label: 'CloudShip', icon: Cloud, path: '/cloudship' },
  { id: 'settings', label: 'Settings', icon: Settings, path: '/settings' },
];

export const Sidebar: React.FC<SidebarProps> = ({ currentPage, onPageChange }) => {
  return (
    <div className="w-64 bg-gray-900 text-white p-4 flex flex-col">
      <div className="mb-8">
        <h1 className="text-2xl font-bold mb-2">Station</h1>
        <p className="text-gray-400 text-sm">agents for engineers. Be in control</p>
      </div>

      <nav className="flex-1">
        <ul className="space-y-2">
          {sidebarItems.map((item) => {
            const Icon = item.icon;
            return (
              <li key={item.id}>
                <button
                  onClick={() => onPageChange(item.id)}
                  className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                    currentPage === item.id
                      ? 'bg-blue-600 text-white'
                      : 'text-gray-300 hover:bg-gray-800 hover:text-white'
                  }`}
                >
                  <Icon size={20} />
                  <span>{item.label}</span>
                </button>
              </li>
            );
          })}
        </ul>
      </nav>
    </div>
  );
};