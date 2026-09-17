'use client';

import { useState, useCallback } from 'react';
import { commentService } from '@/services/kanban';
import { useKanbanStore } from '@/stores/kanban';
import type { CreateCommentData, UpdateCommentData, TaskComment } from '@/types/kanban';

interface UseCommentsReturn {
    isLoading: boolean;
    error: string | null;
    getCommentsByTask: (taskId: string) => ReturnType<typeof useKanbanStore.getState>['comments'];
    loadComments: (taskId: string) => Promise<void>;
    addComment: (taskId: string, data: CreateCommentData) => Promise<TaskComment>;
    updateComment: (commentId: string, data: UpdateCommentData) => Promise<void>;
    deleteComment: (commentId: string) => Promise<void>;
}

export const useComments = (projectId: string): UseCommentsReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const {
        comments,
        addCommentToStore,
        updateCommentInStore,
        deleteCommentFromStore,
        setComments,
    } = useKanbanStore();

    const getCommentsByTask = useCallback(
        (taskId: string) => comments.filter(comment => comment.taskId === taskId),
        [comments]
    );

    const loadComments = useCallback(async (taskId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            const taskComments = await commentService.getComments(projectId, taskId);
            const otherComments = comments.filter(comment => comment.taskId !== taskId);
            setComments([...otherComments, ...taskComments]);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load comments');
        } finally {
            setIsLoading(false);
        }
    }, [projectId, comments, setComments]);

    const addComment = useCallback(async (taskId: string, data: CreateCommentData): Promise<TaskComment> => {
        setIsLoading(true);
        setError(null);

        try {
            const comment = await commentService.createComment(projectId, taskId, data);
            addCommentToStore(comment);
            return comment;
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to add comment');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, addCommentToStore]);

    const updateComment = useCallback(async (commentId: string, data: UpdateCommentData): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            const comment = await commentService.updateComment(projectId, commentId, data);
            updateCommentInStore(comment);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to update comment');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, updateCommentInStore]);

    const deleteComment = useCallback(async (commentId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            await commentService.deleteComment(projectId, commentId);
            deleteCommentFromStore(commentId);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to delete comment');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, deleteCommentFromStore]);

    return {
        isLoading,
        error,
        getCommentsByTask,
        loadComments,
        addComment,
        updateComment,
        deleteComment,
    };
};
