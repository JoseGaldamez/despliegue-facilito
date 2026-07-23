import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiClient, ApiError } from '../lib/api';
import type { GitHubRepo } from '../types/dtos';

type Step = 'select-repo' | 'configure';

export default function NewProjectPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>('select-repo');

  // Step 1 state
  const [repos, setRepos] = useState<GitHubRepo[]>([]);
  const [loadingRepos, setLoadingRepos] = useState(true);
  const [filter, setFilter] = useState('');
  const [selectedRepo, setSelectedRepo] = useState<GitHubRepo | null>(null);

  // Step 2 state
  const [subdomain, setSubdomain] = useState('');
  const [packageManager, setPackageManager] = useState('npm');
  const [buildCommand, setBuildCommand] = useState('');
  const [outputDir, setOutputDir] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadRepos() {
      try {
        const response = await apiClient.getGitHubRepos();
        setRepos(response.repos);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'No se pudieron cargar los repositorios.');
      } finally {
        setLoadingRepos(false);
      }
    }
    loadRepos();
  }, []);

  const filteredRepos = repos.filter(
    (r) =>
      r.name.toLowerCase().includes(filter.toLowerCase()) ||
      r.fullName.toLowerCase().includes(filter.toLowerCase())
  );

  function handleRepoSelect(repo: GitHubRepo) {
    setSelectedRepo(repo);
    setSubdomain(repo.name.toLowerCase().replace(/[^a-z0-9-]/g, '-'));
    setStep('configure');
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedRepo) return;
    setError(null);
    setIsSubmitting(true);

    try {
      const response = await apiClient.createProject({
        repoUrl: selectedRepo.htmlUrl,
        subdomain,
        packageManager,
        buildCommand: buildCommand || undefined,
        outputDir: outputDir || undefined,
      });
      navigate(`/projects/${response.project.projectId}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Error al crear el proyecto.');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="py-6 max-w-2xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <button
          onClick={() => step === 'configure' ? setStep('select-repo') : navigate('/projects')}
          className="text-gray-400 hover:text-gray-600 transition-colors"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <h1 className="text-2xl font-bold text-gray-900">
          {step === 'select-repo' ? 'Selecciona un repositorio' : 'Configura tu proyecto'}
        </h1>
      </div>

      {/* Step 1: Select Repo */}
      {step === 'select-repo' && (
        <div>
          {loadingRepos ? (
            <div className="flex items-center justify-center py-12">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-green border-t-transparent" />
              <span className="ml-2 text-sm text-gray-500">Cargando repositorios...</span>
            </div>
          ) : (
            <>
              <input
                type="text"
                placeholder="Buscar repositorio..."
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="block w-full rounded-lg border border-gray-200 px-4 py-3 text-sm text-gray-900 placeholder:text-gray-400 focus:border-brand-green focus:ring-1 focus:ring-brand-green focus:outline-none"
              />

              <div className="mt-4 border border-gray-200 rounded-xl overflow-hidden">
                {filteredRepos.length === 0 ? (
                  <p className="p-6 text-sm text-gray-500 text-center">No se encontraron repositorios.</p>
                ) : (
                  <ul className="divide-y divide-gray-100 max-h-[60vh] overflow-y-auto">
                    {filteredRepos.map((repo) => (
                      <li key={repo.fullName}>
                        <button
                          type="button"
                          onClick={() => handleRepoSelect(repo)}
                          className="w-full text-left px-5 py-4 hover:bg-green-50 transition-colors flex items-center justify-between"
                        >
                          <div>
                            <span className="text-sm font-medium text-gray-900">{repo.fullName}</span>
                            {repo.description && (
                              <p className="mt-0.5 text-xs text-gray-500 truncate max-w-md">{repo.description}</p>
                            )}
                          </div>
                          <div className="flex items-center gap-2">
                            {repo.private && (
                              <span className="text-xs bg-gray-100 text-gray-500 rounded px-2 py-0.5">privado</span>
                            )}
                            <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                            </svg>
                          </div>
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </>
          )}

          {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
        </div>
      )}

      {/* Step 2: Configure */}
      {step === 'configure' && selectedRepo && (
        <form onSubmit={handleSubmit} className="space-y-6">
          {/* Selected repo info */}
          <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">{selectedRepo.fullName}</p>
              <p className="text-xs text-gray-500">{selectedRepo.htmlUrl}</p>
            </div>
            <button
              type="button"
              onClick={() => setStep('select-repo')}
              className="text-xs text-brand-green hover:text-brand-green-dark font-medium"
            >
              Cambiar
            </button>
          </div>

          {/* Subdomain */}
          <div>
            <label htmlFor="subdomain" className="block text-sm font-medium text-gray-700">
              Subdominio
            </label>
            <div className="mt-1 flex rounded-lg border border-gray-200 overflow-hidden focus-within:border-brand-green focus-within:ring-1 focus-within:ring-brand-green">
              <input
                id="subdomain"
                type="text"
                value={subdomain}
                onChange={(e) => setSubdomain(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                required
                className="flex-1 px-3 py-2.5 text-sm text-gray-900 focus:outline-none"
                placeholder="mi-proyecto"
              />
              <span className="inline-flex items-center px-3 bg-gray-50 text-sm text-gray-500 border-l border-gray-200">
                .despliegue-facilito.app
              </span>
            </div>
            <p className="mt-1 text-xs text-gray-500">
              URL de tu proyecto: {subdomain || '...'}.despliegue-facilito.app
            </p>
          </div>

          {/* Package Manager */}
          <div>
            <label htmlFor="packageManager" className="block text-sm font-medium text-gray-700">
              Package Manager
            </label>
            <select
              id="packageManager"
              value={packageManager}
              onChange={(e) => setPackageManager(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 px-3 py-2.5 text-sm text-gray-900 focus:border-brand-green focus:ring-1 focus:ring-brand-green focus:outline-none"
            >
              <option value="npm">npm</option>
              <option value="pnpm">pnpm</option>
              <option value="yarn">yarn</option>
              <option value="bun">bun</option>
            </select>
          </div>

          {/* Build Command */}
          <div>
            <label htmlFor="buildCommand" className="block text-sm font-medium text-gray-700">
              Build Command <span className="text-gray-400">(opcional)</span>
            </label>
            <input
              id="buildCommand"
              type="text"
              value={buildCommand}
              onChange={(e) => setBuildCommand(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 px-3 py-2.5 text-sm text-gray-900 placeholder:text-gray-400 focus:border-brand-green focus:ring-1 focus:ring-brand-green focus:outline-none"
              placeholder={`${packageManager} run build (auto-detectado)`}
            />
          </div>

          {/* Output Directory */}
          <div>
            <label htmlFor="outputDir" className="block text-sm font-medium text-gray-700">
              Directorio de salida <span className="text-gray-400">(opcional)</span>
            </label>
            <input
              id="outputDir"
              type="text"
              value={outputDir}
              onChange={(e) => setOutputDir(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 px-3 py-2.5 text-sm text-gray-900 placeholder:text-gray-400 focus:border-brand-green focus:ring-1 focus:ring-brand-green focus:outline-none"
              placeholder="dist (auto-detectado)"
            />
          </div>

          {error && <p className="text-sm text-red-600">{error}</p>}

          {/* Actions */}
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => navigate('/projects')}
              className="rounded-lg border border-gray-200 px-4 py-2.5 text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors"
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isSubmitting || !subdomain}
              className="rounded-lg bg-brand-green px-5 py-2.5 text-sm font-semibold text-white hover:bg-brand-green-dark disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {isSubmitting ? 'Desplegando...' : 'Desplegar proyecto'}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
