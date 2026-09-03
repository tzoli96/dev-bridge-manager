'use client';

import React from 'react';
import { Button } from '@/components/ui/button';
import { Select } from '@/components/ui/select';
import { boardService, kanbanService, taskService } from '@/services/kanban';
import type { Board, KanbanColumn as ColumnType } from '@/types/kanban';

interface AddToBoardModalProps {
    projectId: string;
    currentBoardId: string;
    taskId: string;
    onDone: () => void;
}

export const AddToBoardModal: React.FC<AddToBoardModalProps> = ({ projectId, currentBoardId, taskId, onDone }) => {
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [selectedBoardId, setSelectedBoardId] = React.useState('');
    const [columns, setColumns] = React.useState<ColumnType[]>([]);
    const [selectedColumnId, setSelectedColumnId] = React.useState('');
    const [isLoading, setIsLoading] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);

    React.useEffect(() => {
        boardService.listBoards(projectId).then((all) => {
            const targets = all.filter((b) => b.id !== currentBoardId);
            setBoards(targets);
            if (targets.length > 0) setSelectedBoardId(targets[0].id);
        });
    }, [projectId, currentBoardId]);

    React.useEffect(() => {
        if (!selectedBoardId) {
            setColumns([]);
            setSelectedColumnId('');
            return;
        }
        kanbanService.getBoard(projectId, selectedBoardId).then((board) => {
            setColumns(board.columns);
            setSelectedColumnId(board.columns[0]?.id ?? '');
        });
    }, [projectId, selectedBoardId]);

    const handleSubmit = async () => {
        if (!selectedBoardId || !selectedColumnId) return;
        setIsLoading(true);
        setError(null);
        try {
            await taskService.placeTask(projectId, selectedBoardId, taskId, { columnId: selectedColumnId });
            onDone();
        } catch {
            setError('Failed to add task to board.');
        } finally {
            setIsLoading(false);
        }
    };

    if (boards.length === 0) {
        return <p className="text-sm text-gray-500">No other boards in this project yet.</p>;
    }

    return (
        <div className="space-y-4">
            {error && (
                <div className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            <Select
                label="Board"
                value={selectedBoardId}
                onChange={setSelectedBoardId}
                options={boards.map((b) => ({ value: b.id, label: b.name }))}
            />

            <Select
                label="Column"
                value={selectedColumnId}
                onChange={setSelectedColumnId}
                options={columns.map((c) => ({ value: c.id, label: c.title }))}
            />

            <div className="flex items-center justify-end gap-3 pt-4 border-t">
                <Button type="button" variant="ghost" onClick={onDone}>Cancel</Button>
                <Button type="button" onClick={handleSubmit} loading={isLoading} disabled={!selectedColumnId}>
                    Add to board
                </Button>
            </div>
        </div>
    );
};
