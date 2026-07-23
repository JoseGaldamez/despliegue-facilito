import { useState, useEffect, type FormEvent } from 'react';
import { apiClient } from '../lib/api';
import type { Domain } from '../types/api';

interface DomainManagerProps {
  projectId: string;
}

export function DomainManager({ projectId }: DomainManagerProps) {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hostname, setHostname] = useState('');
  const [adding, setAdding] = useState(false);
  const [addedHostname, setAddedHostname] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    fetchDomains();
  }, [projectId]);

  async function fetchDomains() {
    setLoading(true);
    setError(null);
    try {
      const response = await apiClient.getDomains(projectId);
      setDomains(response.domains);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load domains');
    } finally {
      setLoading(false);
    }
  }

  async function handleAddDomain(e: FormEvent) {
    e.preventDefault();
    const trimmed = hostname.trim();
    if (!trimmed) return;

    setAdding(true);
    setError(null);
    setAddedHostname(null);
    try {
      const response = await apiClient.addDomain(projectId, trimmed);
      setDomains((prev) => [...prev, response.domain]);
      setAddedHostname(trimmed);
      setHostname('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add domain');
    } finally {
      setAdding(false);
    }
  }

  async function handleDeleteDomain(domainId: string) {
    setDeletingId(domainId);
    setError(null);
    try {
      await apiClient.deleteDomain(projectId, domainId);
      setDomains((prev) => prev.filter((d) => d.domainId !== domainId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete domain');
    } finally {
      setDeletingId(null);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-600" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold text-gray-900">Custom Domains</h2>

      {error && (
        <div className="rounded-md bg-red-50 p-4" role="alert">
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}

      {/* Add domain form */}
      <form onSubmit={handleAddDomain} className="flex gap-3">
        <input
          type="text"
          value={hostname}
          onChange={(e) => setHostname(e.target.value)}
          placeholder="app.example.com"
          className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-gray-900 placeholder-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          aria-label="Domain hostname"
          disabled={adding}
        />
        <button
          type="submit"
          disabled={adding || !hostname.trim()}
          className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {adding ? 'Adding...' : 'Add Domain'}
        </button>
      </form>

      {/* DNS instructions panel */}
      {addedHostname && (
        <div className="rounded-md border border-blue-200 bg-blue-50 p-4 space-y-3">
          <h3 className="text-sm font-semibold text-blue-900">Configure your DNS</h3>
          <p className="text-sm text-blue-800">
            Add a CNAME record pointing your domain to your CloudFront distribution:
          </p>
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm" aria-label="DNS configuration">
              <thead>
                <tr className="border-b border-blue-200">
                  <th className="px-3 py-2 text-left font-medium text-blue-900">Type</th>
                  <th className="px-3 py-2 text-left font-medium text-blue-900">Name</th>
                  <th className="px-3 py-2 text-left font-medium text-blue-900">Value</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td className="px-3 py-2 text-blue-800">CNAME</td>
                  <td className="px-3 py-2 text-blue-800 font-mono">{addedHostname}</td>
                  <td className="px-3 py-2 text-blue-800 font-mono">d1234.cloudfront.net</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p className="text-xs text-blue-700">
            DNS changes may take up to 48 hours to propagate.
          </p>
          <button
            onClick={() => setAddedHostname(null)}
            className="text-xs text-blue-600 hover:text-blue-800 underline"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Domains list */}
      {domains.length === 0 ? (
        <p className="text-sm text-gray-500 py-4">
          No custom domains configured yet. Add one above to get started.
        </p>
      ) : (
        <ul className="divide-y divide-gray-200 rounded-md border border-gray-200" role="list">
          {domains.map((domain) => (
            <li
              key={domain.domainId}
              className="flex items-center justify-between px-4 py-3"
            >
              <span className="text-sm font-mono text-gray-900">{domain.hostname}</span>
              <button
                onClick={() => handleDeleteDomain(domain.domainId)}
                disabled={deletingId === domain.domainId}
                className="rounded-md px-3 py-1 text-sm text-red-600 hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
                aria-label={`Delete domain ${domain.hostname}`}
              >
                {deletingId === domain.domainId ? 'Deleting...' : 'Delete'}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
