'use client';

import { useState, useCallback } from 'react';
import { timeEntryService, taskService } from '@/services/kanban';
import { useKanbanStore } from '@/stores/kanban';
import type { CreateTimeEntryData } from '@/types/kanban';

interface UseTimeEntriesReturn {
    isLoading: boolean;
    error: string | null;
    getEntriesByTask: (taskId: string) => ReturnType<typeof useKanbanStore.getState>['timeEntries'];
    getTotalHoursByTask: (taskId: string) => number;
    loadTimeEntries: (taskId: string) => Promise<void>;
    addTimeEntry: (taskId: string, data: CreateTimeEntryData) => Promise<void>;
    updateTaskEstimate: (taskId: string, estimatedHours: number) => Promise<void>;
}

export const useTimeEntries = (projectId: string): UseTimeEntriesReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const {
        timeEntries,
        addTimeEntryToStore,
        setTimeEntries,
        updateTaskInStore,
    } = useKanbanStore();

    const getEntriesByTask = useCallback(
        (taskId: string) => timeEntries.filter(entry => entry.taskId === taskId),
        [timeEntries]
    );

    const getTotalHoursByTask = useCallback(
        (taskId: string) => timeEntries
            .filter(entry => entry.taskId === taskId)
            .reduce((total, entry) => total + entry.hours, 0),
        [timeEntries]
    );

    const loadTimeEntries = useCallback(async (taskId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            const entries = await timeEntryService.getTimeEntries(projectId, taskId);
            const otherEntries = timeEntries.filter(entry => entry.taskId !== taskId);
            setTimeEntries([...otherEntries, ...entries]);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load time entries');
        } finally {
            setIsLoading(false);
        }
    }, [projectId, timeEntries, setTimeEntries]);

    const addTimeEntry = useCallback(async (taskId: string, data: CreateTimeEntryData): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            const entry = await timeEntryService.createTimeEntry(projectId, taskId, data);
            addTimeEntryToStore(entry);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to add time entry');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, addTimeEntryToStore]);

    const updateTaskEstimate = useCallback(async (taskId: string, estimatedHours: number): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            const updated = await taskService.updateTask(projectId, taskId, { estimatedHours });
            updateTaskInStore(updated);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to update estimate');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, updateTaskInStore]);

    return {
        isLoading,
        error,
        getEntriesByTask,
        getTotalHoursByTask,
        loadTimeEntries,
        addTimeEntry,
        updateTaskEstimate,
    };
};
