'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { KanbanProvider } from '@/providers/kanban-provider';
import { KanbanBoard } from '@/components/kanban';

export default function BoardPage() {
    const { projectId, boardId } = useParams<{ projectId: string; boardId: string }>();

    return (
        <div className="space-y-4 p-6">
            <Link href={`/dashboard/board/${projectId}`} className="text-sm text-blue-600 hover:text-blue-700">
                ← Back to boards
            </Link>
            <KanbanProvider projectId={projectId} boardId={boardId}>
                <KanbanBoard />
            </KanbanProvider>
        </div>
    );
}
