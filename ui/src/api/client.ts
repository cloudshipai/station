import axios from 'axios';

// Station API client configuration
// In development mode (Vite dev server), we use relative URLs to leverage the proxy
// In production mode (built assets served by Station), we use the full URL
const isDev = import.meta.env.DEV;
const API_BASE_URL = isDev ? '/api/v1' : 'http://localhost:8585/api/v1';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 30000, // Increased timeout for workflow operations
});

// Request interceptor for adding auth tokens if needed
apiClient.interceptors.request.use(
  (config) => {
    // No authentication required for local Station instance
    // Station running with --local flag doesn't require API keys
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Response interceptor for handling errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Handle unauthorized access
      localStorage.removeItem('station_token');
      // Could redirect to login or show auth modal
    }
    return Promise.reject(error);
  }
);