'use client';

import { useState, useEffect, useMemo } from 'react';
import { useAuth } from '@/hooks/auth/use-auth';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useProject } from '@/hooks/projects/use-project';
import { kanbanService } from '@/services/kanban';
import type {
    KanbanBoard,
    KanbanColumn,
    KanbanPermissions,
    Task
} from '@/types/kanban';

interface UseKanbanOptions {
    autoRefresh?: boolean;
    refreshInterval?: number;
}

interface UseKanbanReturn {
    board: KanbanBoard | null;
    columns: KanbanColumn[];
    isLoading: boolean;
    error: string | null;
    permissions: KanbanPermissions;
    refresh: () => Promise<void>;
    updateBoard: (updates: Partial<KanbanBoard>) => Promise<void>;
}

export const useKanban = (
    projectId: string,
    options: UseKanbanOptions = {}
): UseKanbanReturn => {
    const { autoRefresh = false, refreshInterval = 30000 } = options;

    // Meglévő hook-ok használata
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const { project } = useProject(projectId);

    // State
    const [board, setBoard] = useState<KanbanBoard | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Permission-ök kiszámítása
    const permissions = useMemo((): KanbanPermissions => ({
        canCreateTasks: hasPermission('tasks:create', projectId),
        canEditTasks: hasPermission('tasks:edit', projectId),
        canDeleteTasks: hasPermission('tasks:delete', projectId),
        canMoveTasks: hasPermission('tasks:move', projectId),
        canManageColumns: hasPermission('kanban:manage_columns', projectId),
        canViewTimeTracking: hasPermission('time_tracking:view', projectId),
        canEditTimeTracking: hasPermission('time_tracking:edit', projectId),
    }), [hasPermission, projectId]);

    // Board betöltése
    const loadBoard = async (): Promise<void> => {
        try {
            setError(null);
            const boardData = await kanbanService.getBoard(projectId);
            setBoard(boardData);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load kanban board');
        } finally {
            setIsLoading(false);
        }
    };

    // Board frissítése
    const refresh = async (): Promise<void> => {
        await loadBoard();
    };

    // Board beállítások frissítése
    const updateBoard = async (updates: Partial<KanbanBoard>): Promise<void> => {
        if (!board || !permissions.canManageColumns) return;

        try {
            const updatedBoard = await kanbanService.updateBoard(projectId, updates);
            setBoard(updatedBoard);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to update board');
        }
    };

    // Kezdeti betöltés
    useEffect(() => {
        if (projectId && user) {
            loadBoard();
        }
    }, [projectId, user]);

    // Auto refresh
    useEffect(() => {
        if (!autoRefresh || !projectId) return;

        const interval = setInterval(() => {
            loadBoard();
        }, refreshInterval);

        return () => clearInterval(interval);
    }, [autoRefresh, refreshInterval, projectId]);

    return {
        board,
        columns: board?.columns || [],
        isLoading,
        error,
        permissions,
        refresh,
        updateBoard,
    };
};