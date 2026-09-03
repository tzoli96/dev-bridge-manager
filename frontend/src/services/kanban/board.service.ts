import { apiClient } from '@/lib/api';
import type { Board } from '@/types/kanban';

/**
 * Board entity service
 * Single Responsibility: board CRUD API operations
 */
export const boardService = {
    async listBoards(projectId: string): Promise<Board[]> {
        return apiClient.get(`/projects/${projectId}/boards`);
    },

    async createBoard(projectId: string, data: { name: string; position: number }): Promise<Board> {
        return apiClient.post(`/projects/${projectId}/boards`, data);
    },

    async updateBoard(projectId: string, boardId: string, data: { name?: string; position?: number }): Promise<Board> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}`, data);
    },

    async deleteBoard(projectId: string, boardId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/boards/${boardId}`);
    }
};
