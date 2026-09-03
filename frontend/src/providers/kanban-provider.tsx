'use client';

import React, { createContext, useContext, useEffect } from 'react';
import { useKanbanStore } from '@/stores/kanban/kanban.store';
import { kanbanService } from '@/services/kanban';

interface KanbanContextValue {
    projectId: string;
    boardId: string;
    isLoading: boolean;
    error: string | null;
}

const KanbanContext = createContext<KanbanContextValue | null>(null);

interface KanbanProviderProps {
    projectId: string;
    boardId: string;
    children: React.ReactNode;
}

export const KanbanProvider: React.FC<KanbanProviderProps> = ({
                                                                  projectId,
                                                                  boardId,
                                                                  children
                                                              }) => {
    const {
        setBoard,
        setTasks,
        setComments,
        setTimeEntries,
        setLoading,
        setError,
        isLoading,
        error
    } = useKanbanStore();

    useEffect(() => {
        const loadKanbanData = async () => {
            if (!projectId || !boardId) return;

            setLoading(true);
            setError(null);

            try {
                const board = await kanbanService.getBoard(projectId, boardId);
                const tasks = board.columns.flatMap((col) => col.tasks);

                setBoard(board);
                setTasks(tasks);
                setComments(tasks.flatMap((task) => task.comments ?? []));
                setTimeEntries(tasks.flatMap((task) => task.timeEntries ?? []));
            } catch (err) {
                console.error('Error loading kanban data:', err);
                setError(err instanceof Error ? err.message : 'Failed to load board data');
            } finally {
                setLoading(false);
            }
        };

        loadKanbanData();
    }, [projectId, boardId, setBoard, setTasks, setComments, setTimeEntries, setLoading, setError]);

    const contextValue: KanbanContextValue = {
        projectId,
        boardId,
        isLoading,
        error
    };

    return (
        <KanbanContext.Provider value={contextValue}>
            {children}
        </KanbanContext.Provider>
    );
};

export const useKanbanContext = (): KanbanContextValue => {
    const context = useContext(KanbanContext);
    if (!context) {
        throw new Error('useKanbanContext must be used within KanbanProvider');
    }
    return context;
};
