import React, { useState, useEffect } from 'react';
import { environmentsApi } from '../api/station';

export const EnvironmentContext = React.createContext<any>({
  environments: [],
  selectedEnvironment: null,
  setSelectedEnvironment: () => {},
  refreshTrigger: 0,
  refreshData: () => {}
});

export const EnvironmentProvider = ({ children }: { children: React.ReactNode }) => {
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(null);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  // Fetch environments
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const envs = response.data.environments || [];
        setEnvironments(envs);
        if (envs.length > 0 && !selectedEnvironment) {
          // Default to the first environment if none is selected
          setSelectedEnvironment(envs[0].id);
        }
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]);
      }
    };
    fetchEnvironments();
  }, [refreshTrigger]);

  const environmentContext = {
    environments,
    selectedEnvironment,
    setSelectedEnvironment,
    refreshTrigger,
    refreshData: () => setRefreshTrigger(prev => prev + 1)
  };

  return (
    <EnvironmentContext.Provider value={environmentContext}>
      {children}
    </EnvironmentContext.Provider>
  );
};
