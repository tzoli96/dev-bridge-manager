import { apiClient } from '@/lib/api';
import type {
    Task,
    CreateTaskData,
    UpdateTaskData,
    MoveTaskData,
    TaskFilters
} from '@/types/kanban';

/**
 * Task service
 * Single Responsibility: Task CRUD operations
 */
export const taskService = {
    /**
     * Get all tasks for a project
     */
    async getTasks(
        projectId: string,
        filters?: TaskFilters
    ): Promise<Task[]> {
        const params = new URLSearchParams();

        if (filters?.assigneeIds?.length) {
            filters.assigneeIds.forEach(id => params.append('assigneeId', id));
        }

        if (filters?.priorities?.length) {
            filters.priorities.forEach(priority => params.append('priority', priority));
        }

        if (filters?.tags?.length) {
            filters.tags.forEach(tag => params.append('tag', tag));
        }

        if (filters?.search) {
            params.append('search', filters.search);
        }

        if (filters?.hasEstimate !== undefined) {
            params.append('hasEstimate', filters.hasEstimate.toString());
        }

        if (filters?.isOverdue !== undefined) {
            params.append('isOverdue', filters.isOverdue.toString());
        }

        const queryString = params.toString();
        const url = `/projects/${projectId}/tasks${queryString ? `?${queryString}` : ''}`;

        return apiClient.get(url);
    },

    /**
     * Get single task
     */
    async getTask(projectId: string, taskId: string): Promise<Task> {
        return apiClient.get(`/projects/${projectId}/tasks/${taskId}`);
    },

    /**
     * Create new task
     */
    async createTask(
        projectId: string,
        data: CreateTaskData
    ): Promise<Task> {
        return apiClient.post(`/projects/${projectId}/tasks`, {
            ...data,
            projectId // Ensure projectId is included
        });
    },

    /**
     * Update task
     */
    async updateTask(
        projectId: string,
        taskId: string,
        data: UpdateTaskData
    ): Promise<Task> {
        return apiClient.put(`/projects/${projectId}/tasks/${taskId}`, data);
    },

    /**
     * Delete task
     */
    async deleteTask(projectId: string, taskId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/tasks/${taskId}`);
    },

    /**
     * Move task to different column
     */
    async moveTask(
        projectId: string,
        taskId: string,
        data: MoveTaskData
    ): Promise<Task> {
        return apiClient.put(`/projects/${projectId}/tasks/${taskId}/move`, data);
    },

    /**
     * Bulk update tasks
     */
    async bulkUpdateTasks(
        projectId: string,
        updates: { taskId: string; data: UpdateTaskData }[]
    ): Promise<Task[]> {
        return apiClient.put(`/projects/${projectId}/tasks/bulk`, { updates });
    },

    /**
     * Duplicate task
     */
    async duplicateTask(projectId: string, taskId: string): Promise<Task> {
        return apiClient.post(`/projects/${projectId}/tasks/${taskId}/duplicate`);
    },

    /**
     * Archive task
     */
    async archiveTask(projectId: string, taskId: string): Promise<Task> {
        return apiClient.put(`/projects/${projectId}/tasks/${taskId}/archive`);
    },

    /**
     * Restore archived task
     */
    async restoreTask(projectId: string, taskId: string): Promise<Task> {
        return apiClient.put(`/projects/${projectId}/tasks/${taskId}/restore`);
    }
};
