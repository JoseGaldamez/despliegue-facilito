export interface User {
  uid: string;
  email: string;
  displayName: string;
  githubUsername?: string;
}

export interface Project {
  projectId: string;
  userId: string;
  repoUrl: string;
  subdomain: string;
  deployUrl: string;
  framework: 'react' | 'angular' | 'vue' | 'astro' | 'html';
  packageManager: string;
  buildCommand: string;
  outputDir: string;
  deploymentStatus: 'pending' | 'building' | 'deployed' | 'failed';
  failureReason?: string;
  currentBuildId?: string;
  distributionId?: string;
  createdAt: string; // ISO 8601
  updatedAt: string;
}

export interface Domain {
  domainId: string;
  projectId: string;
  hostname: string;
  createdAt: string;
}

export interface Build {
  buildId: string;
  projectId: string;
  status: 'in_progress' | 'succeeded' | 'failed';
  logGroupName: string;
  logStreamName: string;
  startedAt: string;
  completedAt?: string;
}
