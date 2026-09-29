// frontend/src/services/passwordsService.ts
import { apiClient } from '@/lib/api';

export interface ProjectPassword {
    id: number;
    project_id: number;
    title: string;
    username: string;
    password: string;
    url: string;
    notes: string;
    created_by: number;
    created_by_name: string;
    created_at: string;
    updated_at: string;
}

export interface ProjectPasswordInput {
    title: string;
    username: string;
    password: string;
    url: string;
    notes: string;
}

export const passwordsService = {
    async list(projectId: string | number): Promise<ProjectPassword[]> {
        return apiClient.get(`/projects/${projectId}/passwords`);
    },

    async create(projectId: string | number, data: ProjectPasswordInput): Promise<ProjectPassword> {
        return apiClient.post(`/projects/${projectId}/passwords`, data);
    },

    async update(
        projectId: string | number,
        passwordId: number,
        data: ProjectPasswordInput
    ): Promise<ProjectPassword> {
        return apiClient.put(`/projects/${projectId}/passwords/${passwordId}`, data);
    },

    async remove(projectId: string | number, passwordId: number): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/passwords/${passwordId}`);
    },
};
