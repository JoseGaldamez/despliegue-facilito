import { auth } from './firebase';
import type {
  ProjectListResponse,
  ProjectResponse,
  DomainListResponse,
  DomainResponse,
  GitHubConnectionResponse,
  GitHubStatusResponse,
  GitHubReposResponse,
} from '../types/dtos';

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

class ApiClient {
  private baseUrl: string;

  constructor() {
    this.baseUrl = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';
  }

  private async getAuthToken(): Promise<string> {
    const user = auth.currentUser;
    if (!user) {
      throw new ApiError(401, 'User is not authenticated');
    }
    return user.getIdToken();
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const token = await this.getAuthToken();

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
        ...options.headers,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      let message = `Request failed with status ${response.status}`;
      try {
        const parsed = JSON.parse(errorBody);
        if (parsed.message) message = parsed.message;
      } catch {
        // use default message
      }
      throw new ApiError(response.status, message);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return response.json() as Promise<T>;
  }

  async getProjects(): Promise<ProjectListResponse> {
    return this.request<ProjectListResponse>('/api/v1/projects');
  }

  async createProject(data: { repoUrl: string; subdomain: string; packageManager: string; buildCommand?: string; outputDir?: string }): Promise<ProjectResponse> {
    return this.request<ProjectResponse>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getProject(projectId: string): Promise<ProjectResponse> {
    return this.request<ProjectResponse>(`/api/v1/projects/${projectId}`);
  }

  async getDomains(projectId: string): Promise<DomainListResponse> {
    return this.request<DomainListResponse>(`/api/v1/projects/${projectId}/domains`);
  }

  async addDomain(projectId: string, hostname: string): Promise<DomainResponse> {
    return this.request<DomainResponse>(`/api/v1/projects/${projectId}/domains`, {
      method: 'POST',
      body: JSON.stringify({ hostname }),
    });
  }

  async deleteDomain(projectId: string, domainId: string): Promise<void> {
    return this.request<void>(`/api/v1/projects/${projectId}/domains/${domainId}`, {
      method: 'DELETE',
    });
  }

  async deleteProject(projectId: string): Promise<void> {
    return this.request<void>(`/api/v1/projects/${projectId}`, {
      method: 'DELETE',
    });
  }

  async connectGitHub(code: string): Promise<GitHubConnectionResponse> {
    return this.request<GitHubConnectionResponse>('/api/v1/github/connect', {
      method: 'POST',
      body: JSON.stringify({ code }),
    });
  }

  async getGitHubStatus(): Promise<GitHubStatusResponse> {
    return this.request<GitHubStatusResponse>('/api/v1/github/status');
  }

  async getGitHubRepos(): Promise<GitHubReposResponse> {
    return this.request<GitHubReposResponse>('/api/v1/github/repos');
  }
}

export const apiClient = new ApiClient();
export { ApiError };
