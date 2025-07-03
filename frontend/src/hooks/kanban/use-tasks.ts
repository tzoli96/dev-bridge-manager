'use client';

import { useState, useCallback, useMemo } from 'react';
import { taskService } from '@/services/kanban';
import { useKanbanStore } from '@/stores/kanban';
import type {
    Task,
    CreateTaskData,
    UpdateTaskData,
    MoveTaskData,
    TaskFilters,
    TaskSortOptions
} from '@/types/kanban';

interface UseTasksReturn {
    tasks: Task[];
    filteredTasks: Task[];
    isLoading: boolean;
    error: string | null;
    createTask: (data: CreateTaskData) => Promise<Task>;
    updateTask: (taskId: string, data: UpdateTaskData) => Promise<Task>;
    deleteTask: (taskId: string) => Promise<void>;
    moveTask: (taskId: string, data: MoveTaskData) => Promise<Task>;
    setFilters: (filters: TaskFilters) => void;
    setSortOptions: (options: TaskSortOptions) => void;
    getTasksByColumn: (columnId: string) => Task[];
}

export const useTasks = (projectId: string): UseTasksReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [filters, setFilters] = useState<TaskFilters>({});
    const [sortOptions, setSortOptions] = useState<TaskSortOptions>({
        field: 'createdAt',
        direction: 'desc'
    });

    // Zustand store használata
    const {
        tasks,
        addTask,
        updateTaskInStore,
        deleteTaskFromStore,
        moveTaskInStore
    } = useKanbanStore();

    // Szűrt és rendezett feladatok
    const filteredTasks = useMemo(() => {
        let result = tasks.filter(task => task.projectId === projectId);

        // Szűrés
        if (filters.assigneeIds?.length) {
            result = result.filter(task =>
                task.assigneeId && filters.assigneeIds!.includes(task.assigneeId)
            );
        }

        if (filters.priorities?.length) {
            result = result.filter(task =>
                filters.priorities!.includes(task.priority)
            );
        }

        if (filters.tags?.length) {
            result = result.filter(task =>
                task.tags.some(tag => filters.tags!.includes(tag.name))
            );
        }

        if (filters.search) {
            const searchLower = filters.search.toLowerCase();
            result = result.filter(task =>
                task.title.toLowerCase().includes(searchLower) ||
                task.description.toLowerCase().includes(searchLower)
            );
        }

        if (filters.hasEstimate) {
            result = result.filter(task => task.estimatedHours > 0);
        }

        if (filters.isOverdue) {
            const now = new Date();
            result = result.filter(task =>
                task.dueDate && new Date(task.dueDate) < now
            );
        }

        // Rendezés
        return result.sort((a, b) => {
            const { field, direction } = sortOptions;
            let aValue: any, bValue: any;

            switch (field) {
                case 'priority':
                    const priorityOrder = { urgent: 4, high: 3, medium: 2, low: 1 };
                    aValue = priorityOrder[a.priority];
                    bValue = priorityOrder[b.priority];
                    break;
                case 'dueDate':
                    aValue = a.dueDate ? new Date(a.dueDate) : new Date('9999-12-31');
                    bValue = b.dueDate ? new Date(b.dueDate) : new Date('9999-12-31');
                    break;
                default:
                    aValue = a[field];
                    bValue = b[field];
            }

            if (direction === 'asc') {
                return aValue > bValue ? 1 : -1;
            } else {
                return aValue < bValue ? 1 : -1;
            }
        });
    }, [tasks, projectId, filters, sortOptions]);

    // Oszlop szerint csoportosított feladatok
    const getTasksByColumn = useCallback((columnId: string): Task[] => {
        return filteredTasks
            .filter(task => task.columnId === columnId)
            .sort((a, b) => a.position - b.position);
    }, [filteredTasks]);

    // CRUD műveletek
    const createTask = useCallback(async (data: CreateTaskData): Promise<Task> => {
        setIsLoading(true);
        setError(null);

        try {
            const task = await taskService.createTask(projectId, data);
            addTask(task);
            return task;
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Failed to create task';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, addTask]);

    const updateTask = useCallback(async (
        taskId: string,
        data: UpdateTaskData
    ): Promise<Task> => {
        setIsLoading(true);
        setError(null);

        try {
            const task = await taskService.updateTask(projectId, taskId, data);
            updateTaskInStore(task);
            return task;
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Failed to update task';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, updateTaskInStore]);

    const deleteTask = useCallback(async (taskId: string): Promise<void> => {
        setIsLoading(true);
        setError(null);

        try {
            await taskService.deleteTask(projectId, taskId);
            deleteTaskFromStore(taskId);
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Failed to delete task';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, deleteTaskFromStore]);

    const moveTask = useCallback(async (
        taskId: string,
        data: MoveTaskData
    ): Promise<Task> => {
        // Optimistic update
        moveTaskInStore(taskId, data.columnId, data.position);

        try {
            const task = await taskService.moveTask(projectId, taskId, data);
            updateTaskInStore(task);
            return task;
        } catch (err) {
            // Revert optimistic update on error
            // TODO: Implement revert logic
            const errorMessage = err instanceof Error ? err.message : 'Failed to move task';
            setError(errorMessage);
            throw err;
        }
    }, [projectId, moveTaskInStore, updateTaskInStore]);

    return {
        tasks: tasks.filter(task => task.projectId === projectId),
        filteredTasks,
        isLoading,
        error,
        createTask,
        updateTask,
        deleteTask,
        moveTask,
        setFilters,
        setSortOptions,
        getTasksByColumn,
    };
};
