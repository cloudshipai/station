import React, { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { SyncModal } from './SyncModal';
import { CheckCircle, AlertCircle, Terminal, XCircle } from 'lucide-react';

export const SyncPage: React.FC = () => {
  const { env } = useParams<{ env: string }>();
  const [searchParams] = useSearchParams();
  const syncId = searchParams.get('sync_id');
  const [completed, setCompleted] = useState(false);
  const [failed, setFailed] = useState(false);
  const [cancelled, setCancelled] = useState(false);

  const handleCancel = async () => {
    if (!syncId) return;
    try {
      await fetch(`/api/v1/sync/cancel/${syncId}`, { method: 'POST' });
      setCancelled(true);
    } catch (err) {
      console.error('Failed to cancel sync:', err);
    }
  };

  if (!env) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="bg-white rounded-lg shadow-lg p-8 text-center max-w-md">
          <AlertCircle className="h-16 w-16 text-red-500 mx-auto mb-4" />
          <h1 className="text-2xl font-bold text-gray-900 mb-2">Error</h1>
          <p className="text-gray-600">Environment not specified in URL</p>
        </div>
      </div>
    );
  }

  if (completed) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-green-50 to-cyan-50 flex items-center justify-center">
        <div className="bg-white rounded-xl shadow-xl p-10 text-center max-w-lg">
          <div className="w-20 h-20 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-6">
            <CheckCircle className="h-12 w-12 text-green-500" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 mb-3">Sync Complete!</h1>
          <p className="text-gray-600 mb-6 text-lg">
            Variables have been configured and saved to <code className="bg-gray-100 px-2 py-1 rounded text-sm">variables.yml</code>
          </p>
          <div className="bg-gray-50 rounded-lg p-4 mb-6">
            <div className="flex items-center justify-center gap-2 text-gray-700">
              <Terminal className="h-5 w-5" />
              <span>You can close this window and return to the terminal.</span>
            </div>
          </div>
          <p className="text-sm text-gray-500">
            The CLI will automatically detect completion and continue.
          </p>
        </div>
      </div>
    );
  }

  if (failed) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-red-50 to-orange-50 flex items-center justify-center">
        <div className="bg-white rounded-xl shadow-xl p-10 text-center max-w-lg">
          <div className="w-20 h-20 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-6">
            <AlertCircle className="h-12 w-12 text-red-500" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 mb-3">Sync Failed</h1>
          <p className="text-gray-600 mb-6">
            There was an error during the sync operation. Check the terminal for details.
          </p>
          <div className="bg-gray-50 rounded-lg p-4">
            <div className="flex items-center justify-center gap-2 text-gray-700">
              <Terminal className="h-5 w-5" />
              <span>Return to the terminal to see error details.</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (cancelled) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-gray-50 to-slate-100 flex items-center justify-center">
        <div className="bg-white rounded-xl shadow-xl p-10 text-center max-w-lg">
          <div className="w-20 h-20 bg-gray-100 rounded-full flex items-center justify-center mx-auto mb-6">
            <XCircle className="h-12 w-12 text-gray-500" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 mb-3">Sync Cancelled</h1>
          <p className="text-gray-600 mb-6">
            The sync operation was cancelled. No changes were made.
          </p>
          <div className="bg-gray-50 rounded-lg p-4">
            <div className="flex items-center justify-center gap-2 text-gray-700">
              <Terminal className="h-5 w-5" />
              <span>You can close this window and return to the terminal.</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-cyan-50 to-blue-50 flex items-center justify-center p-4">
      <div className="w-full max-w-2xl">
        <div className="text-center mb-6">
          <h1 className="text-2xl font-bold text-gray-900 mb-2">Configure Variables</h1>
          <p className="text-gray-600">
            Environment: <code className="bg-white px-2 py-1 rounded text-cyan-700 font-semibold">{env}</code>
          </p>
          {syncId && (
            <p className="text-xs text-gray-400 mt-2">Session: {syncId}</p>
          )}
        </div>
        <SyncModal
          isOpen={true}
          onClose={handleCancel}
          environment={env}
          autoStart={true}
          syncId={syncId || undefined}
          onSyncComplete={() => setCompleted(true)}
          onSyncFailed={() => setFailed(true)}
          standalone={true}
        />
        <div className="text-center mt-4">
          <button
            onClick={handleCancel}
            className="text-gray-500 hover:text-gray-700 text-sm underline"
          >
            Cancel and return to terminal
          </button>
        </div>
      </div>
    </div>
  );
};
