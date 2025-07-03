// services/kanban/index.ts
export { kanbanService } from './kanban.service';
export { taskService } from './task.service';
export { commentService } from './comment.service';
export { timeEntryService } from './time-entry.service';

// services/kanban/kanban.service.ts
import { apiClient } from '@/lib/api';
import type { KanbanBoard } from '@/types/kanban';

/**
 * Kanban board service
 * Single Responsibility: Kanban board API operations
 */
export const kanbanService = {
    /**
     * Get kanban board for a project
     */
    async getBoard(projectId: string): Promise<KanbanBoard> {
        return apiClient.get(`/projects/${projectId}/kanban`);
    },

    /**
     * Update kanban board settings
     */
    async updateBoard(
        projectId: string,
        updates: Partial<KanbanBoard>
    ): Promise<KanbanBoard> {
        return apiClient.put(`/projects/${projectId}/kanban`, updates);
    },

    /**
     * Create a new column
     */
    async createColumn(projectId: string, data: {
        title: string;
        color: string;
        position: number;
        maxTasks?: number;
    }) {
        return apiClient.post(`/projects/${projectId}/kanban/columns`, data);
    },

    /**
     * Update column
     */
    async updateColumn(
        projectId: string,
        columnId: string,
        data: {
            title?: string;
            color?: string;
            position?: number;
            maxTasks?: number;
        }
    ) {
        return apiClient.put(`/projects/${projectId}/kanban/columns/${columnId}`, data);
    },

    /**
     * Delete column
     */
    async deleteColumn(projectId: string, columnId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/kanban/columns/${columnId}`);
    },

    /**
     * Reorder columns
     */
    async reorderColumns(projectId: string, columnOrders: {
        columnId: string;
        position: number;
    }[]): Promise<void> {
        return apiClient.put(`/projects/${projectId}/kanban/columns/reorder`, {
            orders: columnOrders
        });
    }
};