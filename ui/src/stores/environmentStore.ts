import { create } from 'zustand';
import type { Environment } from '../types/station';

interface EnvironmentStore {
  environments: Environment[];
  currentEnvironment: Environment | null;
  isLoading: boolean;
  error: string | null;
  
  setEnvironments: (environments: Environment[]) => void;
  setCurrentEnvironment: (environment: Environment | null) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
}

export const useEnvironmentStore = create<EnvironmentStore>((set) => ({
  environments: [],
  currentEnvironment: null,
  isLoading: false,
  error: null,
  
  setEnvironments: (environments) => set({ environments }),
  setCurrentEnvironment: (environment) => set({ currentEnvironment: environment }),
  setLoading: (isLoading) => set({ isLoading }),
  setError: (error) => set({ error }),
}));