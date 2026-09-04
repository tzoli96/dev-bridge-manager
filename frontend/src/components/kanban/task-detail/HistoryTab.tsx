'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useActivityLog } from '@/hooks/kanban/use-activity-log';
import type { ActivityLogEntry } from '@/types/kanban';

const PAGE_SIZE = 20;

const describe = (entry: ActivityLogEntry): string => {
    const who = entry.user?.name ?? 'Someone';
    switch (entry.eventType) {
        case 'field_changed':
            return `${who} changed ${entry.fieldName}: ${entry.oldValue || '—'} → ${entry.newValue || '—'}`;
        case 'moved':
            return `${who} moved the task: ${entry.oldValue} → ${entry.newValue}`;
        case 'comment_added':
            return `${who} commented: “${entry.newValue}”`;
        case 'comment_deleted':
            return `${who} deleted a comment: “${entry.newValue}”`;
        case 'attachment_added':
            return `${who} attached ${entry.newValue}`;
        case 'attachment_deleted':
            return `${who} removed attachment ${entry.newValue}`;
        default:
            return `${who} made a change`;
    }
};

interface HistoryTabProps {
    taskId: string;
}

export const HistoryTab: React.FC<HistoryTabProps> = ({ taskId }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { loadHistory, isLoading, error } = useActivityLog(projectId);
    const [entries, setEntries] = useState<ActivityLogEntry[]>([]);
    const [hasMore, setHasMore] = useState(false);

    const fetchPage = useCallback(async (offset: number) => {
        const page = await loadHistory(taskId, { limit: PAGE_SIZE, offset });
        setEntries((prev) => (offset === 0 ? page : [...prev, ...page]));
        setHasMore(page.length === PAGE_SIZE);
    }, [loadHistory, taskId]);

    useEffect(() => { fetchPage(0); }, [fetchPage]);

    return (
        <div className="space-y-3">
            {entries.map((entry) => (
                <div key={entry.id} className="text-sm border-b pb-2">
                    <p className="text-gray-800">{describe(entry)}</p>
                    <p className="text-xs text-gray-400">{new Date(entry.createdAt).toLocaleString()}</p>
                </div>
            ))}
            {error && <p className="text-sm text-red-600">{error}</p>}
            {hasMore && (
                <button
                    type="button"
                    onClick={() => fetchPage(entries.length)}
                    disabled={isLoading}
                    className="text-sm text-blue-600 hover:text-blue-700"
                >
                    {isLoading ? 'Loading…' : 'Load more'}
                </button>
            )}
        </div>
    );
};
