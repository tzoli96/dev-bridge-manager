// frontend/src/services/kanban/jiraIntegrationService.ts
import { apiClient } from '@/lib/api';
import type { JiraIntegrationStatus, JiraIntegrationConnectData } from '@/types/kanban';

interface StatusApiResponse extends JiraIntegrationStatus {
    success?: boolean;
    message?: string;
}

interface ActionApiResponse {
    success: boolean;
    message?: string;
}

export const jiraIntegrationService = {
    async getStatus(projectId: string, boardId: string): Promise<JiraIntegrationStatus> {
        return apiClient.get(`/projects/${projectId}/boards/${boardId}/jira-integration`);
    },

    async connect(projectId: string, boardId: string, data: JiraIntegrationConnectData): Promise<JiraIntegrationStatus> {
        const response = await apiClient.post<StatusApiResponse>(`/projects/${projectId}/boards/${boardId}/jira-integration`, data);
        if (response.success === false) throw new Error(response.message || 'Failed to connect Jira integration');
        return response;
    },

    async disconnect(projectId: string, boardId: string): Promise<void> {
        const response = await apiClient.delete<ActionApiResponse>(`/projects/${projectId}/boards/${boardId}/jira-integration`);
        if (!response.success) throw new Error(response.message || 'Failed to disconnect Jira integration');
    },
};
