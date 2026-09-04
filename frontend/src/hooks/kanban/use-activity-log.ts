'use client';

import { useState, useCallback } from 'react';
import { activityLogService } from '@/services/kanban';
import type { ActivityLogEntry } from '@/types/kanban';

interface UseActivityLogReturn {
    isLoading: boolean;
    error: string | null;
    loadHistory: (taskId: string, params?: { limit?: number; offset?: number }) => Promise<ActivityLogEntry[]>;
}

export const useActivityLog = (projectId: string): UseActivityLogReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const loadHistory = useCallback(async (taskId: string, params?: { limit?: number; offset?: number }) => {
        setIsLoading(true);
        setError(null);
        try {
            return await activityLogService.getHistory(projectId, taskId, params);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load history');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId]);

    return { isLoading, error, loadHistory };
};
