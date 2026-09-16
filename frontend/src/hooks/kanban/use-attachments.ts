'use client';

import { useState, useCallback } from 'react';
import { attachmentService } from '@/services/kanban';
import { useKanbanStore } from '@/stores/kanban';
import type { TaskAttachment } from '@/types/kanban';

interface UseAttachmentsReturn {
    isLoading: boolean;
    error: string | null;
    uploadAttachments: (taskId: string, files: File[], commentId?: string) => Promise<TaskAttachment[]>;
    deleteAttachment: (taskId: string, attachmentId: string, commentId?: string) => Promise<void>;
}

export const useAttachments = (projectId: string): UseAttachmentsReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const { updateTaskInStore, updateCommentInStore } = useKanbanStore();

    const uploadAttachments = useCallback(async (
        taskId: string,
        files: File[],
        commentId?: string
    ): Promise<TaskAttachment[]> => {
        setIsLoading(true);
        setError(null);
        try {
            const uploaded = await attachmentService.uploadAttachments(projectId, taskId, files, commentId);

            if (commentId) {
                const comment = useKanbanStore.getState().comments.find(c => c.id === commentId);
                if (comment) {
                    updateCommentInStore({ ...comment, attachments: [...comment.attachments, ...uploaded] });
                }
            } else {
                const task = useKanbanStore.getState().tasks.find(t => t.id === taskId);
                if (task) {
                    updateTaskInStore({ ...task, attachments: [...task.attachments, ...uploaded] });
                }
            }

            return uploaded;
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to upload attachments');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, updateTaskInStore, updateCommentInStore]);

    const deleteAttachment = useCallback(async (
        taskId: string,
        attachmentId: string,
        commentId?: string
    ): Promise<void> => {
        setIsLoading(true);
        setError(null);
        try {
            await attachmentService.deleteAttachment(projectId, attachmentId);

            if (commentId) {
                const comment = useKanbanStore.getState().comments.find(c => c.id === commentId);
                if (comment) {
                    updateCommentInStore({ ...comment, attachments: comment.attachments.filter(a => a.id !== attachmentId) });
                }
            } else {
                const task = useKanbanStore.getState().tasks.find(t => t.id === taskId);
                if (task) {
                    updateTaskInStore({ ...task, attachments: task.attachments.filter(a => a.id !== attachmentId) });
                }
            }
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to delete attachment');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, updateTaskInStore, updateCommentInStore]);

    return { isLoading, error, uploadAttachments, deleteAttachment };
};
