import { apiClient } from '@/lib/api';
import type {
    TimeEntry,
    CreateTimeEntryData,
    UpdateTimeEntryData
} from '@/types/kanban';

/**
 * Time entry service
 * Single Responsibility: Time tracking operations
 */
export const timeEntryService = {
    /**
     * Get time entries for a task
     */
    async getTimeEntries(projectId: string, taskId: string): Promise<TimeEntry[]> {
        return apiClient.get(`/projects/${projectId}/tasks/${taskId}/time-entries`);
    },

    /**
     * Get time entries for entire project
     */
    async getProjectTimeEntries(
        projectId: string,
        filters?: {
            userId?: string;
            startDate?: string;
            endDate?: string;
            taskIds?: string[];
        }
    ): Promise<TimeEntry[]> {
        const params = new URLSearchParams();

        if (filters?.userId) params.append('userId', filters.userId);
        if (filters?.startDate) params.append('startDate', filters.startDate);
        if (filters?.endDate) params.append('endDate', filters.endDate);
        if (filters?.taskIds?.length) {
            filters.taskIds.forEach(id => params.append('taskId', id));
        }

        const queryString = params.toString();
        const url = `/projects/${projectId}/time-entries${queryString ? `?${queryString}` : ''}`;

        return apiClient.get(url);
    },

    /**
     * Create new time entry
     */
    async createTimeEntry(
        projectId: string,
        taskId: string,
        data: CreateTimeEntryData
    ): Promise<TimeEntry> {
        return apiClient.post(`/projects/${projectId}/tasks/${taskId}/time-entries`, {
            ...data,
            taskId // Ensure taskId is included
        });
    },

    /**
     * Update time entry
     */
    async updateTimeEntry(
        projectId: string,
        entryId: string,
        data: UpdateTimeEntryData
    ): Promise<TimeEntry> {
        return apiClient.put(`/projects/${projectId}/time-entries/${entryId}`, data);
    },

    /**
     * Delete time entry
     */
    async deleteTimeEntry(projectId: string, entryId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/time-entries/${entryId}`);
    },

    /**
     * Start timer for task
     */
    async startTimer(projectId: string, taskId: string): Promise<TimeEntry> {
        return apiClient.post(`/projects/${projectId}/tasks/${taskId}/timer/start`);
    },

    /**
     * Stop timer for task
     */
    async stopTimer(
        projectId: string,
        taskId: string,
        description?: string
    ): Promise<TimeEntry> {
        return apiClient.post(`/projects/${projectId}/tasks/${taskId}/timer/stop`, {
            description
        });
    },

    /**
     * Get active timer for user
     */
    async getActiveTimer(projectId: string): Promise<TimeEntry | null> {
        return apiClient.get(`/projects/${projectId}/timer/active`);
    },

    /**
     * Get time summary for project
     */
    async getTimeSummary(
        projectId: string,
        filters?: {
            startDate?: string;
            endDate?: string;
            userId?: string;
        }
    ): Promise<{
        totalHours: number;
        billableHours: number;
        taskBreakdown: { taskId: string; hours: number }[];
        userBreakdown: { userId: string; hours: number }[];
    }> {
        const params = new URLSearchParams();

        if (filters?.startDate) params.append('startDate', filters.startDate);
        if (filters?.endDate) params.append('endDate', filters.endDate);
        if (filters?.userId) params.append('userId', filters.userId);

        const queryString = params.toString();
        const url = `/projects/${projectId}/time-entries/summary${queryString ? `?${queryString}` : ''}`;

        return apiClient.get(url);
    }
};