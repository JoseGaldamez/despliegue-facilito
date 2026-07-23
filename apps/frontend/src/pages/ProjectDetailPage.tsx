import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { apiClient, ApiError } from '../lib/api';
import type { Project } from '../types/api';

function StatusBadge({ status }: { status: Project['deploymentStatus'] }) {
  const styles: Record<Project['deploymentStatus'], string> = {
    pending: 'bg-yellow-50 text-yellow-700 border border-yellow-200',
    building: 'bg-blue-50 text-blue-700 border border-blue-200',
    deployed: 'bg-green-50 text-green-700 border border-green-200',
    failed: 'bg-red-50 text-red-700 border border-red-200',
  };

  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-sm font-medium ${styles[status]}`}>
      {status}
    </span>
  );
}

export default function ProjectDetailPage() {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    async function loadProject() {
      if (!projectId) return;
      try {
        const response = await apiClient.getProject(projectId);
        setProject(response.project);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'No se pudo cargar el proyecto.');
      } finally {
        setLoading(false);
      }
    }
    loadProject();
  }, [projectId]);

  // Auto-refresh when building or pending
  useEffect(() => {
    if (!project || (project.deploymentStatus !== 'building' && project.deploymentStatus !== 'pending')) return;

    const interval = setInterval(async () => {
      if (!projectId) return;
      try {
        const response = await apiClient.getProject(projectId);
        setProject(response.project);
      } catch {
        // ignore polling errors
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [project?.deploymentStatus, projectId]);

  async function handleDelete() {
    if (!projectId) return;
    if (!confirm('¿Estás seguro de que quieres eliminar este proyecto? Esta acción no se puede deshacer.')) return;

    setDeleting(true);
    try {
      await apiClient.deleteProject(projectId);
      navigate('/projects');
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'No se pudo eliminar el proyecto.');
      setDeleting(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-brand-green border-t-transparent" />
      </div>
    );
  }

  if (error || !project) {
    return (
      <div className="py-12 text-center">
        <p className="text-sm text-red-600">{error || 'Proyecto no encontrado.'}</p>
        <button
          onClick={() => navigate('/projects')}
          className="mt-3 text-sm text-brand-green hover:text-brand-green-dark font-medium"
        >
          ← Volver a proyectos
        </button>
      </div>
    );
  }

  return (
    <div className="py-6 max-w-4xl mx-auto">
      {/* Back link */}
      <button
        onClick={() => navigate('/projects')}
        className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-brand-green transition-colors mb-6"
      >
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        Proyectos
      </button>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{project.subdomain || project.projectId.slice(0, 8)}</h1>
          <p className="mt-1 text-sm text-gray-500">
            <a
              href={project.repoUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="text-brand-green hover:text-brand-green-dark hover:underline"
            >
              {project.repoUrl.replace('https://github.com/', '')}
            </a>
          </p>
        </div>
        <StatusBadge status={project.deploymentStatus} />
      </div>

      {/* Building indicator */}
      {(project.deploymentStatus === 'building' || project.deploymentStatus === 'pending') && (
        <div className="mt-4 rounded-lg border border-blue-200 bg-blue-50 px-4 py-4">
          <div className="flex items-center gap-3">
            <div className="h-5 w-5 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />
            <div>
              <p className="text-sm font-medium text-blue-800">
                {project.deploymentStatus === 'building' ? 'Construyendo tu proyecto...' : 'Preparando deploy...'}
              </p>
              <p className="text-xs text-blue-600 mt-0.5">
                Esto puede tardar unos minutos. La página se actualiza automáticamente.
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Deploy URL */}
      {project.deployUrl && (
        <div className="mt-4 rounded-lg border border-green-200 bg-green-50 px-4 py-3">
          <p className="text-xs font-medium text-green-600 uppercase tracking-wider">URL de deploy</p>
          <a
            href={project.deployUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium text-brand-green hover:text-brand-green-dark hover:underline"
          >
            {project.deployUrl}
          </a>
        </div>
      )}
      {!project.deployUrl && project.subdomain && (
        <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
          <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">URL de deploy</p>
          <p className="text-sm text-gray-500">Provisionando distribución...</p>
        </div>
      )}

      {/* Failure reason */}
      {project.failureReason && (
        <div className="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3">
          <p className="text-xs font-medium text-red-600 uppercase tracking-wider">Error</p>
          <p className="mt-1 text-sm text-red-700">{project.failureReason}</p>
        </div>
      )}

      {/* Info grid */}
      <div className="mt-6 grid grid-cols-1 sm:grid-cols-2 gap-4">
        <InfoCard label="Project ID" value={project.projectId} mono />
        <InfoCard label="Framework" value={project.framework} />
        <InfoCard label="Package Manager" value={project.packageManager || 'npm'} />
        <InfoCard label="Build Command" value={project.buildCommand || '(auto-detectado)'} mono />
        <InfoCard label="Output Directory" value={project.outputDir || 'dist'} mono />
        <InfoCard label="Creado" value={formatDate(project.createdAt)} />
        <InfoCard label="Última actualización" value={formatDate(project.updatedAt)} />
        {project.currentBuildId && (
          <InfoCard label="Build ID" value={project.currentBuildId} mono />
        )}
      </div>

      {/* Danger zone */}
      <div className="mt-10 border-t border-gray-200 pt-8">
        <h2 className="text-lg font-semibold text-gray-900">Zona de peligro</h2>
        <p className="mt-1 text-sm text-gray-500">
          Una vez que eliminas un proyecto, no hay vuelta atrás. Se eliminará permanentemente.
        </p>
        <button
          onClick={handleDelete}
          disabled={deleting}
          className="mt-4 rounded-lg border border-red-200 bg-white px-4 py-2.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50 transition-colors"
        >
          {deleting ? 'Eliminando...' : 'Eliminar proyecto'}
        </button>
      </div>
    </div>
  );
}

function InfoCard({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white px-4 py-3">
      <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">{label}</p>
      <p className={`mt-1 text-sm text-gray-900 ${mono ? 'font-mono' : ''} break-all`}>{value}</p>
    </div>
  );
}

function formatDate(iso: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return d.toLocaleDateString('es-MX', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}
