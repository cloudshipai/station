import React, { useState } from 'react';
import { X, Package, AlertCircle, CheckCircle, Loader2, Copy } from 'lucide-react';

interface BuildImageModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentName: string;
}

const BuildImageModal: React.FC<BuildImageModalProps> = ({
  isOpen,
  onClose,
  environmentName,
}) => {
  const [isBuilding, setIsBuilding] = useState(false);
  const [buildStatus, setBuildStatus] = useState<'idle' | 'building' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');
  const [imageName, setImageName] = useState(`station-env-${environmentName.toLowerCase()}`);
  const [tag, setTag] = useState('latest');
  const [imageId, setImageId] = useState('');
  const [environmentVariables, setEnvironmentVariables] = useState<Record<string, string>>({});
  const [dockerRunCommand, setDockerRunCommand] = useState('');

  if (!isOpen) return null;

  const handleBuildImage = async () => {
    setIsBuilding(true);
    setBuildStatus('building');
    setErrorMessage('');

    try {
      // TODO: Replace with actual API call
      const response = await fetch('/api/v1/environments/build-image', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          environment: environmentName,
          image_name: imageName,
          tag: tag,
        }),
      });

      if (!response.ok) {
        throw new Error(`Failed to build image: ${response.statusText}`);
      }

      const result = await response.json();

      if (result.image_id) {
        setImageId(result.image_id);
      }
      if (result.environment_variables) {
        setEnvironmentVariables(result.environment_variables);
        generateDockerRunCommand(result.image_id || `${imageName}:${tag}`, result.environment_variables);
      }

      setBuildStatus('success');

    } catch (error) {
      setBuildStatus('error');
      setErrorMessage(error instanceof Error ? error.message : 'Failed to build Docker image');
    } finally {
      setIsBuilding(false);
    }
  };

  const generateDockerRunCommand = (fullImageName: string, envVars: Record<string, string>) => {
    // Base docker run options with Docker-in-Docker support
    const baseOptions = [
      'docker run -it',
      '--privileged',  // Enable privileged mode for Docker-in-Docker
      '-v /var/run/docker.sock:/var/run/docker.sock',  // Mount docker socket for Ship tools
    ];

    // Port mapping for Station API and MCP
    const portOptions = [
      '-p 8585:8585',  // Station API port
      '-p 8586:8586',  // Station MCP port
    ];

    // Essential environment variables for Station
    const essentialEnvVars = [];

    // Use actual encryption key from environment variables or fallback
    const encryptionKey = envVars.STATION_ENCRYPTION_KEY || '1b6ee7dab74f6303396cd5fd5a10a449a9e7ba14e0dc646b61c8e7879b5fd9f4';
    essentialEnvVars.push(`STATION_ENCRYPTION_KEY="${encryptionKey}"`);

    // Provider-based environment variables - only include if they exist in envVars
    const providerEnvVars = [];
    const providerKeys = {
      'OPENAI_API_KEY': 'OpenAI API Key',
      'ANTHROPIC_API_KEY': 'Anthropic API Key',
      'GEMINI_API_KEY': 'Gemini API Key',
      'CLOUDSHIPAI_REGISTRATION_KEY': 'CloudShip Registration Key'
    };

    Object.entries(providerKeys).forEach(([key, description]) => {
      if (envVars[key] && envVars[key].trim() !== '') {
        // Mask sensitive keys for display but use actual values
        const value = envVars[key];
        if (key.includes('API_KEY') && value.length > 8) {
          // Show first 4 and last 4 characters for API keys
          const maskedValue = `${value.substring(0, 4)}...${value.substring(value.length - 4)}`;
          providerEnvVars.push(`${key}="${value}"`); // Use actual value in command
        } else {
          providerEnvVars.push(`${key}="${value}"`);
        }
      }
    });

    // Environment variables from variables.yml and other config
    const userEnvVars = Object.entries(envVars)
      .filter(([key]) =>
        !key.startsWith('STATION_') &&
        !Object.keys(providerKeys).includes(key)
      )
      .map(([key, value]) => `${key}="${value}"`);

    // Container name and command
    const containerOptions = [
      `--name station-${environmentName.toLowerCase()}`,
      `${fullImageName}`,
      'stn serve'
    ];

    // Build the complete command
    const allOptions = [
      ...baseOptions,
      ...portOptions,
      ...essentialEnvVars.map(env => `-e ${env}`),
      ...providerEnvVars.map(env => `-e ${env}`),
      ...userEnvVars.map(env => `-e ${env}`),
      ...containerOptions
    ];

    const command = allOptions.join(' \\\n  ');
    setDockerRunCommand(command);
  };

  const handleClose = () => {
    if (!isBuilding) {
      onClose();
      setBuildStatus('idle');
      setErrorMessage('');
      setImageId('');
      setEnvironmentVariables({});
      setDockerRunCommand('');
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
      <div className="bg-white border border-gray-200 rounded-lg p-6 w-full max-w-md shadow-xl">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-lg font-semibold text-gray-900 flex items-center gap-2">
            <Package className="h-5 w-5" />
            Build Docker Image
          </h2>
          <button
            onClick={handleClose}
            disabled={isBuilding}
            className="text-gray-600 hover:text-gray-900 transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <p className="text-gray-600 text-sm mb-4">
              Build a Docker image for the <span className="text-station-blue font-medium">{environmentName}</span> environment.
            </p>
          </div>

          <div>
            <label className="block text-gray-900 text-sm font-medium mb-2">
              Image Name
            </label>
            <input
              type="text"
              value={imageName}
              onChange={(e) => setImageName(e.target.value)}
              disabled={isBuilding}
              className="w-full px-3 py-2 bg-white border border-gray-300 rounded text-gray-900 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-station-blue focus:border-station-blue disabled:opacity-50"
              placeholder="station-env-name"
            />
          </div>

          <div>
            <label className="block text-gray-900 text-sm font-medium mb-2">
              Tag
            </label>
            <input
              type="text"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              disabled={isBuilding}
              className="w-full px-3 py-2 bg-white border border-gray-300 rounded text-gray-900 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-station-blue focus:border-station-blue disabled:opacity-50"
              placeholder="latest"
            />
          </div>

          {buildStatus === 'error' && (
            <div className="flex items-center gap-2 p-3 bg-red-50 border border-red-200 rounded-lg">
              <AlertCircle className="h-4 w-4 text-red-600 flex-shrink-0" />
              <span className="text-red-600 text-sm font-medium">{errorMessage}</span>
            </div>
          )}

          {buildStatus === 'success' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 p-3 bg-green-50 border border-green-200 rounded-lg">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span className="text-green-600 text-sm font-medium">
                  Docker image built successfully: {imageName}:{tag}
                </span>
              </div>

              {imageId && (
                <div className="space-y-2">
                  <label className="block text-gray-900 text-sm font-medium">
                    Image ID
                  </label>
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={imageId}
                      readOnly
                      className="flex-1 px-3 py-2 bg-gray-50 border border-gray-300 rounded text-gray-900 font-mono text-sm focus:outline-none"
                    />
                    <button
                      onClick={() => navigator.clipboard.writeText(imageId)}
                      className="px-3 py-2 bg-white border border-gray-300 rounded text-gray-900 hover:bg-gray-50 transition-colors"
                      title="Copy Image ID"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              )}

              {dockerRunCommand && (
                <div className="space-y-2">
                  <label className="block text-gray-900 text-sm font-medium">
                    Docker Run Command
                  </label>
                  <div className="flex items-start gap-2">
                    <textarea
                      value={dockerRunCommand}
                      readOnly
                      rows={8}
                      className="flex-1 px-3 py-2 bg-gray-50 border border-gray-300 rounded text-gray-900 font-mono text-xs focus:outline-none resize-none"
                    />
                    <button
                      onClick={() => navigator.clipboard.writeText(dockerRunCommand)}
                      className="px-3 py-2 bg-white border border-gray-300 rounded text-gray-900 hover:bg-gray-50 transition-colors"
                      title="Copy Docker Command"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                  <p className="text-gray-600 text-xs">
                    This command includes all environment variables from your {environmentName} environment.
                  </p>
                </div>
              )}
            </div>
          )}

          <div className="flex gap-3 pt-4">
            <button
              onClick={handleClose}
              disabled={isBuilding}
              className="flex-1 px-4 py-2 bg-white text-gray-700 border border-gray-300 rounded text-sm hover:bg-gray-50 transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              onClick={handleBuildImage}
              disabled={isBuilding || !imageName.trim() || !tag.trim()}
              className="flex-1 px-4 py-2 bg-station-blue text-white rounded text-sm font-medium hover:bg-blue-600 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {isBuilding ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Building...
                </>
              ) : (
                <>
                  <Package className="h-4 w-4" />
                  Build Image
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default BuildImageModal;