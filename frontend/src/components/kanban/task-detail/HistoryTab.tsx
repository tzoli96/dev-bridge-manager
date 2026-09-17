'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { formatDistanceToNow } from 'date-fns';
import { hu } from 'date-fns/locale';
import { History, Pencil, ArrowRightLeft, MessageSquarePlus, MessageSquareX, Paperclip, PaperclipIcon, Loader2 } from 'lucide-react';
import { useActivityLog } from '@/hooks/kanban/use-activity-log';
import type { ActivityLogEntry } from '@/types/kanban';
import { cn } from '@/lib/utils';

const PAGE_SIZE = 20;

const FIELD_LABELS: Record<string, string> = {
    title: 'title',
    description: 'description',
    priority: 'priority',
    dueDate: 'due date',
    estimatedHours: 'estimated hours',
    assignee: 'assignee',
    tags: 'tags',
};

const fieldLabel = (name?: string) => (name ? FIELD_LABELS[name] ?? name : 'a field');

const EVENT_STYLE: Record<ActivityLogEntry['eventType'], { icon: React.ComponentType<{ size?: number; className?: string }>; className: string }> = {
    field_changed: { icon: Pencil, className: 'bg-primary/10 text-primary' },
    moved: { icon: ArrowRightLeft, className: 'bg-primary/10 text-primary' },
    comment_added: { icon: MessageSquarePlus, className: 'bg-success/10 text-success' },
    comment_deleted: { icon: MessageSquareX, className: 'bg-destructive/10 text-destructive' },
    attachment_added: { icon: Paperclip, className: 'bg-success/10 text-success' },
    attachment_deleted: { icon: PaperclipIcon, className: 'bg-destructive/10 text-destructive' },
};

const describe = (entry: ActivityLogEntry): React.ReactNode => {
    const who = entry.user?.name ?? 'Someone';
    switch (entry.eventType) {
        case 'field_changed':
            return <><b>{who}</b> changed {fieldLabel(entry.fieldName)}: {entry.oldValue || '—'} → {entry.newValue || '—'}</>;
        case 'moved':
            return <><b>{who}</b> moved the task: {entry.oldValue} → {entry.newValue}</>;
        case 'comment_added':
            return <><b>{who}</b> commented: “{entry.newValue}”</>;
        case 'comment_deleted':
            return <><b>{who}</b> deleted a comment: “{entry.newValue}”</>;
        case 'attachment_added':
            return <><b>{who}</b> attached {entry.newValue}</>;
        case 'attachment_deleted':
            return <><b>{who}</b> removed attachment {entry.newValue}</>;
        default:
            return <><b>{who}</b> made a change</>;
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

    if (isLoading && entries.length === 0) {
        return (
            <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
                <Loader2 className="w-4 h-4 animate-spin" /> Loading history…
            </div>
        );
    }

    if (!isLoading && entries.length === 0 && !error) {
        return (
            <div className="text-center py-12 text-muted-foreground">
                <History className="w-12 h-12 mx-auto mb-3 text-muted-foreground/50" />
                <p>No activity yet.</p>
                <p className="text-sm">Changes to this task will show up here.</p>
            </div>
        );
    }

    return (
        <div className="space-y-0.5">
            {entries.map((entry, i) => {
                const { icon: Icon, className } = EVENT_STYLE[entry.eventType] ?? EVENT_STYLE.field_changed;
                const isLast = i === entries.length - 1 && !hasMore;
                return (
                    <div key={entry.id} className="flex gap-3">
                        <div className="flex flex-col items-center">
                            <div className={cn('flex items-center justify-center w-7 h-7 rounded-full shrink-0', className)}>
                                <Icon size={14} />
                            </div>
                            {!isLast && <div className="w-px flex-1 bg-border my-1" />}
                        </div>
                        <div className="flex-1 pb-5 min-w-0">
                            <p className="text-sm text-foreground break-words">{describe(entry)}</p>
                            <p
                                className="text-xs text-muted-foreground mt-0.5"
                                title={new Date(entry.createdAt).toLocaleString()}
                            >
                                {formatDistanceToNow(new Date(entry.createdAt), { addSuffix: true, locale: hu })}
                            </p>
                        </div>
                    </div>
                );
            })}
            {error && <p className="text-sm text-destructive">{error}</p>}
            {hasMore && (
                <div className="pl-10">
                    <button
                        type="button"
                        onClick={() => fetchPage(entries.length)}
                        disabled={isLoading}
                        className="text-sm text-primary hover:text-primary/80"
                    >
                        {isLoading ? 'Loading…' : 'Load more'}
                    </button>
                </div>
            )}
        </div>
    );
};
