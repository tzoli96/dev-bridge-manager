import { apiClient } from '@/lib/api';
import type { Board, CreateBoardData, UpdateBoardData } from '@/types/kanban';

/**
 * Board service
 * Single Responsibility: board CRUD operations
 */
export const boardService = {
    async listBoards(projectId: string): Promise<Board[]> {
        return apiClient.get(`/projects/${projectId}/boards`);
    },

    async getBoard(projectId: string, boardId: string): Promise<Board> {
        return apiClient.get(`/projects/${projectId}/boards/${boardId}`);
    },

    async createBoard(
        projectId: string,
        data: CreateBoardData
    ): Promise<Board> {
        return apiClient.post(`/projects/${projectId}/boards`, data);
    },

    async updateBoard(
        projectId: string,
        boardId: string,
        data: UpdateBoardData
    ): Promise<Board> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}`, data);
    },

    async deleteBoard(projectId: string, boardId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/boards/${boardId}`);
    }
};
