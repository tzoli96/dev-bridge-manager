import { apiClient } from '@/lib/api';
import type { Task, CreateTaskData } from '@/types/kanban';
import { taskService } from './task.service';

/**
 * Subtask service
 * Single Responsibility: subtask listing/creation for a parent task
 */
export const subtaskService = {
    async getSubtasks(projectId: string, parentTaskId: string): Promise<Task[]> {
        return apiClient.get(`/projects/${projectId}/tasks/${parentTaskId}/subtasks`);
    },

    async createSubtask(projectId: string, parentTaskId: string, data: CreateTaskData): Promise<Task> {
        return taskService.createTask(projectId, { ...data, parentTaskId });
    },
};
