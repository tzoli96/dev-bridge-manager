'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { KanbanProvider } from '@/providers/kanban-provider';
import { KanbanBoard } from '@/components/kanban';

export default function BoardPage() {
    const { projectId, boardId } = useParams<{ projectId: string; boardId: string }>();

    return (
        <div className="h-full flex flex-col">
            <div className="px-6 pt-4">
                <Link
                    href={`/dashboard/board/${projectId}`}
                    className="text-sm text-gray-500 hover:text-gray-700"
                >
                    ← Back to boards
                </Link>
            </div>
            <KanbanProvider projectId={projectId} boardId={boardId}>
                <KanbanBoard />
            </KanbanProvider>
        </div>
    );
}
