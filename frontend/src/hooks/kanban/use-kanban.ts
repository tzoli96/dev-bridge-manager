'use client';

import { useMemo } from 'react';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useKanbanStore } from '@/stores/kanban';
import { kanbanService } from '@/services/kanban';
import type {
    KanbanBoard,
    KanbanColumn,
    KanbanPermissions,
} from '@/types/kanban';

interface UseKanbanReturn {
    board: KanbanBoard | null;
    columns: KanbanColumn[];
    isLoading: boolean;
    error: string | null;
    permissions: KanbanPermissions;
    updateBoard: (updates: Partial<KanbanBoard>) => Promise<void>;
}

// Board/column/loading state is owned by the KanbanProvider (which loads it
// into the shared store) so this hook reads from the store rather than
// fetching independently.
export const useKanban = (projectId: string, boardId: string): UseKanbanReturn => {
    const { hasPermission } = usePermissions();
    const { board, columns, isLoading, error, updateBoard: updateBoardInStore } = useKanbanStore();

    const permissions = useMemo((): KanbanPermissions => ({
        canCreateTasks: hasPermission('tasks:create', projectId),
        canEditTasks: hasPermission('tasks:edit', projectId),
        canDeleteTasks: hasPermission('tasks:delete', projectId),
        canMoveTasks: hasPermission('tasks:move', projectId),
        canManageColumns: hasPermission('kanban:manage_columns', projectId),
        canViewTimeTracking: hasPermission('time_tracking:view', projectId),
        canEditTimeTracking: hasPermission('time_tracking:edit', projectId),
    }), [hasPermission, projectId]);

    const updateBoard = async (updates: Partial<KanbanBoard>): Promise<void> => {
        if (!board || !permissions.canManageColumns) return;

        const updatedBoard = await kanbanService.updateBoard(projectId, boardId, updates);
        updateBoardInStore(updatedBoard);
    };

    return {
        board,
        columns,
        isLoading,
        error,
        permissions,
        updateBoard,
    };
};