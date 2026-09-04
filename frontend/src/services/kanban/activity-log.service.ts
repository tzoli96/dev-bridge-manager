import { apiClient } from '@/lib/api';
import type { ActivityLogEntry } from '@/types/kanban';

/**
 * Activity log service
 * Single Responsibility: Task activity/history operations
 */
export const activityLogService = {
    /**
     * Get history entries for a task
     */
    getHistory: (
        projectId: string,
        taskId: string,
        params?: { limit?: number; offset?: number }
    ): Promise<ActivityLogEntry[]> => {
        const query = new URLSearchParams();
        if (params?.limit) query.set('limit', String(params.limit));
        if (params?.offset) query.set('offset', String(params.offset));
        const qs = query.toString();
        return apiClient.get<ActivityLogEntry[]>(
            `/projects/${projectId}/tasks/${taskId}/history${qs ? `?${qs}` : ''}`
        );
    },
};
