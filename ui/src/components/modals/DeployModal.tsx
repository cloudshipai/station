import React, { useState } from 'react';
import { X, Rocket, Download, Cloud, Server, Package, CheckCircle, AlertCircle, Copy, GitBranch, Terminal } from 'lucide-react';

interface DeployModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentId: number;
  environmentName: string;
}

const DeployModal: React.FC<DeployModalProps> = ({
  isOpen,
  onClose,
  environmentId,
  environmentName,
}) => {
  const [selectedProvider, setSelectedProvider] = useState<string>('');
  const [dockerImage, setDockerImage] = useState(`station-${environmentName}:latest`);
  const [isGenerating, setIsGenerating] = useState(false);
  const [generated, setGenerated] = useState(false);
  const [template, setTemplate] = useState('');
  const [filename, setFilename] = useState('');
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);

  if (!isOpen) return null;

  const providers = [
    {
      id: 'cli',
      name: 'CLI (docker exec)',
      icon: Terminal,
      description: 'Bash script for local Docker deployment with agent CLI usage',
    },
    {
      id: 'github-actions',
      name: 'GitHub Actions',
      icon: GitBranch,
      description: 'CI/CD workflow with Docker container and agent execution',
    },
    {
      id: 'aws-ecs',
      name: 'AWS ECS (Fargate)',
      icon: Cloud,
      description: 'CloudFormation template with VPC, ALB, and ECS service',
    },
    {
      id: 'gcp-cloudrun',
      name: 'GCP Cloud Run',
      icon: Cloud,
      description: 'Knative service configuration with secrets management',
    },
    {
      id: 'fly',
      name: 'Fly.io',
      icon: Rocket,
      description: 'fly.toml with volume mounts and auto-scaling',
    },
    {
      id: 'docker-compose',
      name: 'Docker Compose',
      icon: Package,
      description: 'Local/VPS deployment with health checks',
    },
  ];

  const handleGenerate = async () => {
    if (!selectedProvider) {
      setError('Please select a deployment provider');
      return;
    }

    setIsGenerating(true);
    setError('');
    setGenerated(false);

    try {
      const response = await fetch(`/api/v1/environments/${environmentId}/deploy`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          provider: selectedProvider,
          docker_image: dockerImage,
        }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to generate deployment template');
      }

      const result = await response.json();
      setTemplate(result.template);
      setFilename(result.filename);
      setGenerated(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate template');
    } finally {
      setIsGenerating(false);
    }
  };

  const handleDownload = () => {
    const blob = new Blob([template], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(template);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      setError('Failed to copy to clipboard');
    }
  };

  const handleClose = () => {
    setSelectedProvider('');
    setDockerImage(`station-${environmentName}:latest`);
    setGenerated(false);
    setTemplate('');
    setFilename('');
    setError('');
    setCopied(false);
    onClose();
  };

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg border border-gray-200 w-full max-w-4xl max-h-[90vh] flex flex-col shadow-xl">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <div className="flex items-center space-x-3">
            <Rocket className="h-5 w-5 text-orange-600" />
            <h2 className="text-lg font-semibold text-gray-900">
              Deploy Environment: {environmentName}
            </h2>
          </div>
          <button
            onClick={handleClose}
            className="p-1 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-600" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {!generated ? (
            <>
              {/* Docker Image Input */}
              <div className="space-y-2">
                <label className="text-sm font-medium text-gray-900">Docker Image</label>
                <input
                  type="text"
                  value={dockerImage}
                  onChange={(e) => setDockerImage(e.target.value)}
                  className="w-full px-4 py-2.5 bg-white border-2 border-gray-300 rounded-lg text-gray-900 focus:outline-none focus:ring-2 focus:ring-station-blue focus:border-station-blue transition-all"
                  placeholder="station-default:latest"
                />
                <p className="text-xs text-gray-600">
                  The Docker image to deploy (e.g., ghcr.io/yourusername/station-{environmentName}:latest)
                </p>
              </div>

              {/* Provider Selection */}
              <div className="space-y-3">
                <label className="text-sm font-medium text-gray-900">Select Deployment Provider</label>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {providers.map((provider) => {
                    const Icon = provider.icon;
                    return (
                      <button
                        key={provider.id}
                        onClick={() => setSelectedProvider(provider.id)}
                        className={`p-4 rounded-lg border-2 transition-all text-left ${
                          selectedProvider === provider.id
                            ? 'border-station-blue bg-blue-50'
                            : 'border-gray-200 bg-white hover:border-station-blue/50 hover:bg-gray-50'
                        }`}
                      >
                        <div className="flex items-start space-x-3">
                          <Icon className={`h-6 w-6 text-gray-600 flex-shrink-0`} />
                          <div className="flex-1 min-w-0">
                            <div className="font-semibold text-gray-900">
                              {provider.name}
                            </div>
                            <div className="text-sm text-gray-600 mt-1">
                              {provider.description}
                            </div>
                          </div>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Error Display */}
              {error && (
                <div className="flex items-center space-x-2 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600">
                  <AlertCircle className="h-5 w-5 flex-shrink-0" />
                  <span className="text-sm">{error}</span>
                </div>
              )}
            </>
          ) : (
            <>
              {/* Success Message */}
              <div className="flex items-center space-x-2 p-3 bg-green-50 border border-green-200 rounded-lg text-green-600">
                <CheckCircle className="h-5 w-5 flex-shrink-0" />
                <span className="text-sm">
                  Deployment template generated successfully!
                </span>
              </div>

              {/* Template Preview */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-900">
                    Generated Template: {filename}
                  </label>
                  <div className="flex items-center space-x-2">
                    <button
                      onClick={handleCopy}
                      className="flex items-center space-x-1 px-3 py-1.5 bg-gray-100 text-gray-900 rounded text-sm hover:bg-gray-200 transition-colors border border-gray-300"
                    >
                      <Copy className="h-3.5 w-3.5" />
                      <span>{copied ? 'Copied!' : 'Copy'}</span>
                    </button>
                    <button
                      onClick={handleDownload}
                      className="flex items-center space-x-1 px-3 py-1.5 bg-station-blue text-white rounded text-sm hover:bg-blue-600 transition-colors"
                    >
                      <Download className="h-3.5 w-3.5" />
                      <span>Download</span>
                    </button>
                  </div>
                </div>
                <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 max-h-96 overflow-auto">
                  <pre className="text-xs text-gray-900 font-mono whitespace-pre-wrap">
                    {template}
                  </pre>
                </div>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end space-x-3 p-4 border-t border-gray-200">
          <button
            onClick={handleClose}
            className="px-4 py-2 bg-white text-gray-700 rounded border border-gray-300 text-sm hover:bg-gray-50 transition-colors"
          >
            {generated ? 'Close' : 'Cancel'}
          </button>
          {!generated && (
            <button
              onClick={handleGenerate}
              disabled={isGenerating || !selectedProvider}
              className="flex items-center space-x-2 px-4 py-2 bg-station-blue text-white rounded text-sm font-medium hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isGenerating ? (
                <>
                  <Server className="h-4 w-4 animate-spin" />
                  <span>Generating...</span>
                </>
              ) : (
                <>
                  <Rocket className="h-4 w-4" />
                  <span>Generate Template</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  );
};

export default DeployModal;
