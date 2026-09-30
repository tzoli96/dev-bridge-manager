import { apiClient } from '@/lib/api';

export interface ProjectEnvironment {
    id: number;
    project_id: number;
    name: string;
    url: string;
    git_repo_url: string;
    created_at: string;
    updated_at: string;
}

export interface ProjectEnvironmentInput {
    name: string;
    url: string;
    git_repo_url: string;
}

export const environmentsService = {
    async list(projectId: string | number): Promise<ProjectEnvironment[]> {
        return apiClient.get(`/projects/${projectId}/environments`);
    },

    async create(projectId: string | number, data: ProjectEnvironmentInput): Promise<ProjectEnvironment> {
        return apiClient.post(`/projects/${projectId}/environments`, data);
    },

    async update(
        projectId: string | number,
        environmentId: number,
        data: ProjectEnvironmentInput
    ): Promise<ProjectEnvironment> {
        return apiClient.put(`/projects/${projectId}/environments/${environmentId}`, data);
    },

    async remove(projectId: string | number, environmentId: number): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/environments/${environmentId}`);
    },
};
