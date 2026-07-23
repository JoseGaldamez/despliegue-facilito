import type { Project, Domain } from './api';

// Projects
export interface CreateProjectRequest {
  repoUrl: string;
  subdomain: string;
  packageManager: string;
  buildCommand?: string;
  outputDir?: string;
}

export interface ProjectResponse {
  project: Project;
}

export interface ProjectListResponse {
  projects: Project[];
}

// Domains
export interface AddDomainRequest {
  hostname: string;
}

export interface DomainResponse {
  domain: Domain;
}

export interface DomainListResponse {
  domains: Domain[];
}

// GitHub
export interface GitHubConnectionResponse {
  username: string;
}

export interface GitHubStatusResponse {
  connected: boolean;
  username?: string;
}

export interface GitHubRepo {
  name: string;
  fullName: string;
  htmlUrl: string;
  description: string;
  private: boolean;
}

export interface GitHubReposResponse {
  repos: GitHubRepo[];
}

// Error Response
export interface ErrorResponse {
  error: string;
  message: string;
}
