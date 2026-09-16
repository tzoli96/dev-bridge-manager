'use client';

import { useState, useCallback } from 'react';
import { subtaskService } from '@/services/kanban';
import type { Task, CreateTaskData } from '@/types/kanban';

interface UseSubtasksReturn {
    subtasks: Task[];
    isLoading: boolean;
    error: string | null;
    loadSubtasks: () => Promise<Task[]>;
    addSubtask: (data: CreateTaskData) => Promise<Task>;
}

export const useSubtasks = (projectId: string, parentTaskId: string): UseSubtasksReturn => {
    const [subtasks, setSubtasks] = useState<Task[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const loadSubtasks = useCallback(async () => {
        setIsLoading(true);
        setError(null);
        try {
            const data = await subtaskService.getSubtasks(projectId, parentTaskId);
            setSubtasks(data);
            return data;
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load subtasks');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, parentTaskId]);

    const addSubtask = useCallback(async (data: CreateTaskData): Promise<Task> => {
        const subtask = await subtaskService.createSubtask(projectId, parentTaskId, data);
        setSubtasks((prev) => [...prev, subtask]);
        return subtask;
    }, [projectId, parentTaskId]);

    return { subtasks, isLoading, error, loadSubtasks, addSubtask };
};
