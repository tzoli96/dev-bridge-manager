'use client';

import React from 'react';
import { ArrowUp, ArrowDown, Trash2, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useKanbanStore } from '@/stores/kanban';
import { kanbanService } from '@/services/kanban';
import type { KanbanColumn } from '@/types/kanban';

interface ColumnSettingsModalProps {
    projectId: string;
    boardId: string;
}

const COLUMN_COLORS = [
    'bg-primary',
    'bg-warning',
    'bg-primary',
    'bg-success',
    'bg-destructive',
    'bg-primary',
    'bg-primary',
    'bg-muted-foreground/30',
];

interface ColumnTitleInputProps {
    title: string;
    onCommit: (title: string) => void;
    className?: string;
}

const ColumnTitleInput: React.FC<ColumnTitleInputProps> = ({ title, onCommit, className }) => {
    const [draft, setDraft] = React.useState(title);

    React.useEffect(() => {
        setDraft(title);
    }, [title]);

    return (
        <Input
            value={draft}
            onChange={setDraft}
            onBlur={() => {
                const trimmed = draft.trim();
                if (trimmed && trimmed !== title) {
                    onCommit(trimmed);
                } else {
                    setDraft(title);
                }
            }}
            className={className}
        />
    );
};

const ColorPicker: React.FC<{ value: string; onChange: (color: string) => void }> = ({ value, onChange }) => (
    <div className="flex gap-1.5">
        {COLUMN_COLORS.map((color) => (
            <button
                key={color}
                type="button"
                aria-label={color}
                onClick={() => onChange(color)}
                className={cn(
                    'w-5 h-5 rounded-full transition-transform hover:scale-110',
                    color,
                    value === color && 'ring-2 ring-offset-1 ring-ring'
                )}
            />
        ))}
    </div>
);

export const ColumnSettingsModal: React.FC<ColumnSettingsModalProps> = ({ projectId, boardId }) => {
    const columns = useKanbanStore((state) => state.columns);
    const setColumns = useKanbanStore((state) => state.setColumns);
    const addColumnToStore = useKanbanStore((state) => state.addColumn);
    const updateColumnInStore = useKanbanStore((state) => state.updateColumn);
    const deleteColumnFromStore = useKanbanStore((state) => state.deleteColumn);

    const [error, setError] = React.useState<string | null>(null);
    const [savingId, setSavingId] = React.useState<string | null>(null);
    const [newTitle, setNewTitle] = React.useState('');
    const [newColor, setNewColor] = React.useState(COLUMN_COLORS[0]);

    const sortedColumns = [...columns].sort((a, b) => a.position - b.position);

    const persistColumn = async (columnId: string, updates: Partial<Pick<KanbanColumn, 'title' | 'color' | 'maxTasks'>>) => {
        setError(null);
        setSavingId(columnId);
        try {
            await kanbanService.updateColumn(projectId, boardId, columnId, updates);
            updateColumnInStore(columnId, updates);
        } catch {
            setError('Failed to update column.');
        } finally {
            setSavingId(null);
        }
    };

    const handleMove = async (index: number, direction: -1 | 1) => {
        const targetIndex = index + direction;
        if (targetIndex < 0 || targetIndex >= sortedColumns.length) return;

        const reordered = [...sortedColumns];
        [reordered[index], reordered[targetIndex]] = [reordered[targetIndex], reordered[index]];
        const orders = reordered.map((col, i) => ({ columnId: col.id, position: i }));

        setError(null);
        try {
            await kanbanService.reorderColumns(projectId, boardId, orders);
            setColumns(reordered.map((col, i) => ({ ...col, position: i })));
        } catch {
            setError('Failed to reorder columns.');
        }
    };

    const handleDelete = async (column: KanbanColumn) => {
        const taskCount = column.tasks?.length ?? 0;
        const confirmMessage = taskCount > 0
            ? `Delete "${column.title}"? ${taskCount} task(s) in it will be permanently deleted if this is their only remaining placement; tasks placed on other boards will survive there.`
            : `Delete "${column.title}"?`;
        if (!window.confirm(confirmMessage)) return;

        setError(null);
        setSavingId(column.id);
        try {
            await kanbanService.deleteColumn(projectId, boardId, column.id);
            deleteColumnFromStore(column.id);
        } catch {
            setError('Failed to delete column.');
        } finally {
            setSavingId(null);
        }
    };

    const handleAdd = async () => {
        if (!newTitle.trim()) return;

        setError(null);
        try {
            const column = await kanbanService.createColumn(projectId, boardId, {
                title: newTitle.trim(),
                color: newColor,
                position: columns.length,
            }) as KanbanColumn;
            addColumnToStore(column);
            setNewTitle('');
            setNewColor(COLUMN_COLORS[0]);
        } catch {
            setError('Failed to create column.');
        }
    };

    return (
        <div className="space-y-4">
            {error && (
                <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            <div className="space-y-3">
                {sortedColumns.map((column, index) => (
                    <div key={column.id} className="border border-border rounded-lg p-3 space-y-3">
                        <div className="flex items-center gap-2">
                            <div className="flex flex-col">
                                <button
                                    type="button"
                                    disabled={index === 0}
                                    onClick={() => handleMove(index, -1)}
                                    className="p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 disabled:hover:bg-transparent transition-colors"
                                >
                                    <ArrowUp size={14} />
                                </button>
                                <button
                                    type="button"
                                    disabled={index === sortedColumns.length - 1}
                                    onClick={() => handleMove(index, 1)}
                                    className="p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 disabled:hover:bg-transparent transition-colors"
                                >
                                    <ArrowDown size={14} />
                                </button>
                            </div>

                            <ColumnTitleInput
                                title={column.title}
                                onCommit={(title) => persistColumn(column.id, { title })}
                                className="flex-1"
                            />

                            <button
                                type="button"
                                disabled={savingId === column.id}
                                onClick={() => handleDelete(column)}
                                className="p-2 rounded-md text-muted-foreground hover:text-destructive hover:bg-destructive/10 disabled:opacity-30 disabled:hover:bg-transparent transition-colors"
                            >
                                <Trash2 size={16} />
                            </button>
                        </div>

                        <div className="flex items-center gap-6 pl-8">
                            <div className="flex items-center gap-2">
                                <label className="text-xs font-medium text-muted-foreground">WIP limit</label>
                                <input
                                    type="number"
                                    min={0}
                                    placeholder="—"
                                    value={column.maxTasks ?? ''}
                                    onChange={(e) => updateColumnInStore(column.id, {
                                        maxTasks: e.target.value === '' ? undefined : Number(e.target.value),
                                    })}
                                    onBlur={(e) => persistColumn(column.id, {
                                        maxTasks: e.target.value === '' ? undefined : Number(e.target.value),
                                    })}
                                    className="w-16 rounded-lg border border-input px-2 py-1.5 text-sm shadow-sm focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                                />
                            </div>

                            <div className="flex items-center gap-2">
                                <label className="text-xs font-medium text-muted-foreground">Color</label>
                                <ColorPicker
                                    value={column.color}
                                    onChange={(color) => persistColumn(column.id, { color })}
                                />
                            </div>
                        </div>
                    </div>
                ))}
            </div>

            <div className="border border-dashed border-input rounded-lg p-3 space-y-3">
                <div className="flex items-center gap-2">
                    <Input
                        value={newTitle}
                        onChange={setNewTitle}
                        placeholder="New column title..."
                        className="flex-1"
                    />
                    <Button icon={Plus} onClick={handleAdd} disabled={!newTitle.trim()}>
                        Add Column
                    </Button>
                </div>
                <div className="flex items-center gap-2 pl-1">
                    <label className="text-xs font-medium text-muted-foreground">Color</label>
                    <ColorPicker value={newColor} onChange={setNewColor} />
                </div>
            </div>
        </div>
    );
};
