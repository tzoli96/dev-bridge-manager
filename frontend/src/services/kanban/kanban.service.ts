// services/kanban/kanban.service.ts
import { apiClient } from '@/lib/api';
import type { KanbanBoard } from '@/types/kanban';

/**
 * Kanban board content service
 * Single Responsibility: board-scoped column/settings API operations
 */
export const kanbanService = {
    async getBoard(projectId: string, boardId: string): Promise<KanbanBoard> {
        return apiClient.get(`/projects/${projectId}/boards/${boardId}/kanban`);
    },

    async updateBoard(
        projectId: string,
        boardId: string,
        updates: Partial<KanbanBoard>
    ): Promise<KanbanBoard> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/kanban`, updates);
    },

    async createColumn(projectId: string, boardId: string, data: {
        title: string;
        color: string;
        position: number;
        maxTasks?: number;
    }) {
        return apiClient.post(`/projects/${projectId}/boards/${boardId}/kanban/columns`, data);
    },

    async updateColumn(
        projectId: string,
        boardId: string,
        columnId: string,
        data: {
            title?: string;
            color?: string;
            position?: number;
            maxTasks?: number;
            isDone?: boolean;
        }
    ) {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/kanban/columns/${columnId}`, data);
    },

    async deleteColumn(projectId: string, boardId: string, columnId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/boards/${boardId}/kanban/columns/${columnId}`);
    },

    async reorderColumns(projectId: string, boardId: string, columnOrders: {
        columnId: string;
        position: number;
    }[]): Promise<void> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/kanban/columns/reorder`, {
            orders: columnOrders
        });
    }
};