import { apiClient } from '@/lib/api';
import type {
    TaskComment,
    CreateCommentData,
    UpdateCommentData
} from '@/types/kanban';

/**
 * Comment service
 * Single Responsibility: Task comment operations
 */
export const commentService = {
    /**
     * Get comments for a task
     */
    async getComments(projectId: string, taskId: string): Promise<TaskComment[]> {
        return apiClient.get(`/projects/${projectId}/tasks/${taskId}/comments`);
    },

    /**
     * Create new comment
     */
    async createComment(
        projectId: string,
        taskId: string,
        data: CreateCommentData
    ): Promise<TaskComment> {
        return apiClient.post(`/projects/${projectId}/tasks/${taskId}/comments`, {
            ...data,
            taskId // Ensure taskId is included
        });
    },

    /**
     * Update comment
     */
    async updateComment(
        projectId: string,
        commentId: string,
        data: UpdateCommentData
    ): Promise<TaskComment> {
        return apiClient.put(`/projects/${projectId}/comments/${commentId}`, data);
    },

    /**
     * Delete comment
     */
    async deleteComment(projectId: string, commentId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/comments/${commentId}`);
    },

    /**
     * React to comment (like/unlike)
     */
    async reactToComment(
        projectId: string,
        commentId: string,
        reaction: 'like' | 'unlike'
    ): Promise<TaskComment> {
        return apiClient.post(`/projects/${projectId}/comments/${commentId}/react`, {
            reaction
        });
    },

    /**
     * Pin/unpin comment
     */
    async togglePinComment(
        projectId: string,
        commentId: string
    ): Promise<TaskComment> {
        return apiClient.put(`/projects/${projectId}/comments/${commentId}/pin`);
    }
};