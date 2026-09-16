import { apiClient } from '@/lib/api';
import type { TaskAttachment } from '@/types/kanban';

export const attachmentService = {
    async getAttachments(projectId: string, taskId: string): Promise<TaskAttachment[]> {
        return apiClient.get(`/projects/${projectId}/tasks/${taskId}/attachments`);
    },

    async uploadAttachments(
        projectId: string,
        taskId: string,
        files: File[],
        commentId?: string,
        onProgress?: (progress: number) => void
    ): Promise<TaskAttachment[]> {
        return apiClient.uploadFiles(
            `/projects/${projectId}/tasks/${taskId}/attachments`,
            files,
            commentId ? { commentId } : undefined,
            onProgress
        );
    },

    // downloadUrl comes straight from the attachment's own `downloadUrl` field
    // (already the correct relative path — see buildAttachmentDTO), so no
    // URL-building is duplicated here.
    async downloadAttachment(downloadUrl: string): Promise<Blob> {
        return apiClient.getBlob(downloadUrl);
    },

    async deleteAttachment(projectId: string, attachmentId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/attachments/${attachmentId}`);
    },
};
