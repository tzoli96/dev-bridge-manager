'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { boardService } from '@/services/kanban';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, ArrowLeft, KanbanSquare } from 'lucide-react';
import type { Board } from '@/types/kanban';

export default function BoardsListPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [isLoading, setIsLoading] = React.useState(true);
    const [newBoardName, setNewBoardName] = React.useState('');
    const [isCreating, setIsCreating] = React.useState(false);

    React.useEffect(() => {
        boardService.listBoards(projectId)
            .then((data) => setBoards(data))
            .finally(() => setIsLoading(false));
    }, [projectId]);

    const handleCreate = async () => {
        if (!newBoardName.trim()) return;
        setIsCreating(true);
        try {
            const board = await boardService.createBoard(projectId, {
                name: newBoardName.trim(),
                position: boards.length,
            });
            setBoards((prev) => [...prev, board]);
            setNewBoardName('');
        } finally {
            setIsCreating(false);
        }
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

    return (
        <div className="p-6">
            <Link href="/dashboard/board" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
                <ArrowLeft size={14} /> Back to projects
            </Link>

            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold text-foreground">Boards</h1>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                {boards.map((board) => (
                    <button
                        key={board.id}
                        onClick={() => router.push(`/dashboard/board/${projectId}/${board.id}`)}
                        className="flex items-center gap-3 p-4 bg-card border border-border rounded-lg shadow-sm hover:shadow-sm hover:border-primary/40 transition-all text-left"
                    >
                        <KanbanSquare className="text-primary" size={20} />
                        <span className="font-medium text-foreground">{board.name}</span>
                    </button>
                ))}
            </div>

            <div className="flex items-center gap-2 max-w-sm">
                <Input
                    value={newBoardName}
                    onChange={setNewBoardName}
                    placeholder="New board name..."
                    className="flex-1"
                />
                <Button icon={Plus} onClick={handleCreate} disabled={!newBoardName.trim()} loading={isCreating}>
                    New board
                </Button>
            </div>
        </div>
    );
}
