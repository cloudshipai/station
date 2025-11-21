import React, { useState, useEffect } from 'react';
import { environmentsApi } from '../api/station';

const STORAGE_KEY = 'station_selected_environment';

export const EnvironmentContext = React.createContext<any>({
  environments: [],
  selectedEnvironment: null,
  setSelectedEnvironment: () => {},
  refreshTrigger: 0,
  refreshData: () => {}
});

export const EnvironmentProvider = ({ children }: { children: React.ReactNode }) => {
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(() => {
    // Try to load from localStorage on initialization
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored ? parseInt(stored, 10) : null;
  });
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  // Fetch environments
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const envs = response.data.environments || [];
        setEnvironments(envs);
        
        // If we have a stored environment, verify it still exists
        if (selectedEnvironment) {
          const storedEnvExists = envs.some((env: any) => env.id === selectedEnvironment);
          if (!storedEnvExists && envs.length > 0) {
            // Stored environment no longer exists, default to first
            setSelectedEnvironment(envs[0].id);
            localStorage.setItem(STORAGE_KEY, envs[0].id.toString());
          }
        } else if (envs.length > 0) {
          // No stored environment, default to first
          setSelectedEnvironment(envs[0].id);
          localStorage.setItem(STORAGE_KEY, envs[0].id.toString());
        }
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]);
      }
    };
    fetchEnvironments();
  }, [refreshTrigger]);

  // Persist selection to localStorage whenever it changes
  const handleSetSelectedEnvironment = (envId: number | null) => {
    setSelectedEnvironment(envId);
    if (envId !== null) {
      localStorage.setItem(STORAGE_KEY, envId.toString());
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  };

  const environmentContext = {
    environments,
    selectedEnvironment,
    setSelectedEnvironment: handleSetSelectedEnvironment,
    refreshTrigger,
    refreshData: () => setRefreshTrigger(prev => prev + 1)
  };

  return (
    <EnvironmentContext.Provider value={environmentContext}>
      {children}
    </EnvironmentContext.Provider>
  );
};
