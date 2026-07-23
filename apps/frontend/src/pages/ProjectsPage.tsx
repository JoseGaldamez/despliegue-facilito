import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiClient, ApiError } from '../lib/api';
import type { Project } from '../types/api';

type ViewState = 'loading' | 'error' | 'no-github' | 'empty' | 'list';

function StatusBadge({ status }: { status: Project['deploymentStatus'] }) {
  const styles: Record<Project['deploymentStatus'], string> = {
    pending: 'bg-yellow-50 text-yellow-700 border border-yellow-200',
    building: 'bg-blue-50 text-blue-700 border border-blue-200',
    deployed: 'bg-green-50 text-green-700 border border-green-200',
    failed: 'bg-red-50 text-red-700 border border-red-200',
  };

  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${styles[status]}`}>
      {status}
    </span>
  );
}

function FrameworkBadge({ framework }: { framework: Project['framework'] }) {
  const styles: Record<Project['framework'], string> = {
    react: 'bg-cyan-50 text-cyan-700',
    angular: 'bg-red-50 text-red-700',
    vue: 'bg-emerald-50 text-emerald-700',
    astro: 'bg-purple-50 text-purple-700',
    html: 'bg-gray-50 text-gray-700',
  };

  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${styles[framework]}`}>
      {framework}
    </span>
  );
}

function GitHubConnectBanner({ onConnected }: { onConnected: () => void }) {
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleOAuthCallback = useCallback(async (code: string) => {
    setConnecting(true);
    setError(null);

    const url = new URL(window.location.href);
    url.searchParams.delete('code');
    window.history.replaceState({}, '', url.toString());

    try {
      await apiClient.connectGitHub(code);
      onConnected();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to connect GitHub account.';
      setError(message);
      setConnecting(false);
    }
  }, [onConnected]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    if (code) {
      void handleOAuthCallback(code);
    }
  }, [handleOAuthCallback]);

  const handleConnect = () => {
    const clientId = import.meta.env.VITE_GITHUB_CLIENT_ID;
    if (!clientId) {
      setError('GitHub Client ID no configurado. Agrega VITE_GITHUB_CLIENT_ID a tu archivo .env.');
      return;
    }

    const redirectUri = window.location.origin + window.location.pathname;
    const authUrl = `https://github.com/login/oauth/authorize?client_id=${clientId}&scope=repo&redirect_uri=${encodeURIComponent(redirectUri)}`;
    window.location.href = authUrl;
  };

  if (connecting) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-green border-t-transparent" />
        <span className="ml-2 text-sm text-gray-500">Conectando con GitHub...</span>
      </div>
    );
  }

  return (
    <div className="py-12">
      <div className="text-center max-w-md mx-auto">
        <div className="mx-auto w-16 h-16 rounded-full bg-gray-100 flex items-center justify-center">
          <svg className="h-9 w-9 text-gray-500" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
          </svg>
        </div>
        <h3 className="mt-4 text-lg font-semibold text-gray-900">Conecta tu cuenta de GitHub</h3>
        <p className="mt-2 text-sm text-gray-500">
          Para crear proyectos y desplegar tus aplicaciones, necesitas conectar tu cuenta de GitHub primero.
        </p>

        {error && <p className="mt-3 text-sm text-red-600">{error}</p>}

        <div className="mt-6">
          <button
            type="button"
            onClick={handleConnect}
            className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-5 py-3 text-sm font-semibold text-white hover:bg-gray-700 transition-colors"
          >
            <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
            </svg>
            Conectar GitHub
          </button>
        </div>
      </div>
    </div>
  );
}

export default function ProjectsPage() {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [viewState, setViewState] = useState<ViewState>('loading');
  const [errorMessage, setErrorMessage] = useState('');
  const [githubUsername, setGithubUsername] = useState<string | null>(null);

  useEffect(() => {
    checkGitHubAndLoadProjects();
  }, []);

  async function checkGitHubAndLoadProjects() {
    setViewState('loading');
    try {
      const githubStatus = await apiClient.getGitHubStatus();
      if (!githubStatus.connected) {
        setViewState('no-github');
        return;
      }
      setGithubUsername(githubStatus.username ?? null);

      const response = await apiClient.getProjects();
      setProjects(response.projects);
      setViewState(response.projects.length === 0 ? 'empty' : 'list');
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage('Failed to load projects. Please try again.');
      }
      setViewState('error');
    }
  }

  function handleGitHubConnected() {
    checkGitHubAndLoadProjects();
  }

  if (viewState === 'loading') {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <div className="mx-auto h-8 w-8 animate-spin rounded-full border-4 border-brand-green border-t-transparent" />
          <p className="mt-3 text-sm text-gray-500">Cargando...</p>
        </div>
      </div>
    );
  }

  if (viewState === 'error') {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <p className="text-sm text-red-600">{errorMessage}</p>
          <button
            onClick={checkGitHubAndLoadProjects}
            className="mt-3 rounded-lg bg-brand-green px-4 py-2 text-sm font-medium text-white hover:bg-brand-green-dark transition-colors"
          >
            Reintentar
          </button>
        </div>
      </div>
    );
  }

  if (viewState === 'no-github') {
    return (
      <div className="py-6">
        <h1 className="text-2xl font-bold text-gray-900">Proyectos</h1>
        <GitHubConnectBanner onConnected={handleGitHubConnected} />
      </div>
    );
  }

  if (viewState === 'empty') {
    return (
      <div className="py-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold text-gray-900">Proyectos</h1>
          {githubUsername && (
            <span className="inline-flex items-center gap-1.5 text-sm text-gray-500">
              <span className="h-2 w-2 rounded-full bg-brand-green" />
              @{githubUsername}
            </span>
          )}
        </div>

        <div className="mt-12 text-center">
          <div className="mx-auto w-14 h-14 rounded-full bg-gray-100 flex items-center justify-center">
            <svg className="h-7 w-7 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
            </svg>
          </div>
          <h3 className="mt-3 text-sm font-semibold text-gray-900">Sin proyectos aún</h3>
          <p className="mt-1 text-sm text-gray-500">
            Empieza desplegando tu primer repositorio.
          </p>
          <div className="mt-6">
            <button
              onClick={() => navigate('/projects/new')}
              className="inline-flex items-center rounded-lg bg-brand-green px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-green-dark transition-colors"
            >
              + Nuevo Proyecto
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="py-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Proyectos</h1>
        <div className="flex items-center gap-4">
          {githubUsername && (
            <span className="inline-flex items-center gap-1.5 text-sm text-gray-500">
              <span className="h-2 w-2 rounded-full bg-brand-green" />
              @{githubUsername}
            </span>
          )}
          <button
            onClick={() => navigate('/projects/new')}
            className="inline-flex items-center rounded-lg bg-brand-green px-3 py-2 text-sm font-semibold text-white hover:bg-brand-green-dark transition-colors"
          >
            + Nuevo Proyecto
          </button>
        </div>
      </div>

      <div className="mt-6 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
        <table className="min-w-full divide-y divide-gray-100">
          <thead className="bg-gray-50">
            <tr>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Proyecto
              </th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Repositorio
              </th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Framework
              </th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Status
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {projects.map((project) => (
              <tr
                key={project.projectId}
                onClick={() => navigate(`/projects/${project.projectId}`)}
                className="hover:bg-gray-50 cursor-pointer transition-colors"
              >
                <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">
                  {project.subdomain || project.projectId.slice(0, 8)}
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                  {project.repoUrl.replace('https://github.com/', '')}
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm">
                  <FrameworkBadge framework={project.framework} />
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm">
                  <StatusBadge status={project.deploymentStatus} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
