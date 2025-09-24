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
    const envFlags = Object.entries(envVars)
      .map(([key, value]) => `-e ${key}="${value}"`)
      .join(' ');

    const command = `docker run -d ${envFlags} --name station-${environmentName.toLowerCase()} ${fullImageName}`;
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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-dark1 border border-tokyo-dark4 rounded-lg p-6 w-full max-w-md">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-lg font-mono font-semibold text-tokyo-orange flex items-center gap-2">
            <Package className="h-5 w-5" />
            Build Docker Image
          </h2>
          <button
            onClick={handleClose}
            disabled={isBuilding}
            className="text-tokyo-comment hover:text-tokyo-fg transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <p className="text-tokyo-comment text-sm mb-4">
              Build a Docker image for the <span className="text-tokyo-blue font-medium">{environmentName}</span> environment.
            </p>
          </div>

          <div>
            <label className="block text-tokyo-fg text-sm font-medium mb-2">
              Image Name
            </label>
            <input
              type="text"
              value={imageName}
              onChange={(e) => setImageName(e.target.value)}
              disabled={isBuilding}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-tokyo-purple focus:border-tokyo-purple disabled:opacity-50"
              placeholder="station-env-name"
            />
          </div>

          <div>
            <label className="block text-tokyo-fg text-sm font-medium mb-2">
              Tag
            </label>
            <input
              type="text"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              disabled={isBuilding}
              className="w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-tokyo-purple focus:border-tokyo-purple disabled:opacity-50"
              placeholder="latest"
            />
          </div>

          {buildStatus === 'error' && (
            <div className="flex items-center gap-2 p-3 bg-red-900 bg-opacity-30 border border-red-500 border-opacity-50 rounded">
              <AlertCircle className="h-4 w-4 text-red-400 flex-shrink-0" />
              <span className="text-white text-sm font-medium">{errorMessage}</span>
            </div>
          )}

          {buildStatus === 'success' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 p-3 bg-green-900 bg-opacity-30 border border-green-500 border-opacity-50 rounded">
                <CheckCircle className="h-4 w-4 text-green-400 flex-shrink-0" />
                <span className="text-white text-sm font-medium">
                  Docker image built successfully: {imageName}:{tag}
                </span>
              </div>

              {imageId && (
                <div className="space-y-2">
                  <label className="block text-tokyo-fg text-sm font-medium">
                    Image ID
                  </label>
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={imageId}
                      readOnly
                      className="flex-1 px-3 py-2 bg-gray-800 border border-gray-600 rounded text-white font-mono text-sm focus:outline-none"
                    />
                    <button
                      onClick={() => navigator.clipboard.writeText(imageId)}
                      className="px-3 py-2 bg-tokyo-dark2 border border-tokyo-dark4 rounded text-tokyo-fg hover:bg-tokyo-dark3 transition-colors"
                      title="Copy Image ID"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              )}

              {dockerRunCommand && (
                <div className="space-y-2">
                  <label className="block text-tokyo-fg text-sm font-medium">
                    Docker Run Command
                  </label>
                  <div className="flex items-start gap-2">
                    <textarea
                      value={dockerRunCommand}
                      readOnly
                      rows={3}
                      className="flex-1 px-3 py-2 bg-gray-800 border border-gray-600 rounded text-white font-mono text-xs focus:outline-none resize-none"
                    />
                    <button
                      onClick={() => navigator.clipboard.writeText(dockerRunCommand)}
                      className="px-3 py-2 bg-tokyo-dark2 border border-tokyo-dark4 rounded text-tokyo-fg hover:bg-tokyo-dark3 transition-colors"
                      title="Copy Docker Command"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                  <p className="text-tokyo-comment text-xs">
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
              className="flex-1 px-4 py-2 bg-tokyo-dark2 text-tokyo-fg border border-tokyo-dark4 rounded font-mono text-sm hover:bg-tokyo-dark3 transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              onClick={handleBuildImage}
              disabled={isBuilding || !imageName.trim() || !tag.trim()}
              className="flex-1 px-4 py-2 bg-tokyo-purple text-tokyo-bg rounded font-mono text-sm font-medium hover:bg-opacity-90 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
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