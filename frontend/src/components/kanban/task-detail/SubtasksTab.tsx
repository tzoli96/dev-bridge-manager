'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import { Plus, ListTree, User } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useSubtasks } from '@/hooks/kanban';
import { useTaskAssignees } from '@/hooks/kanban/use-task-assignees';
import { useKanbanStore } from '@/stores/kanban';
import { boardService, kanbanService } from '@/services/kanban';
import type { Board, KanbanColumn, TaskPriority } from '@/types/kanban';
import { cn } from '@/lib/utils';

interface SubtasksTabProps {
    taskId: string;
    onOpenSubtask?: (subtaskId: string) => void;
}

export const SubtasksTab: React.FC<SubtasksTabProps> = ({ taskId, onOpenSubtask }) => {
    const { projectId, boardId } = useParams<{ projectId: string; boardId: string }>();
    const { subtasks, isLoading, error, loadSubtasks, addSubtask } = useSubtasks(projectId, taskId);
    const { assignees, loadAssignees } = useTaskAssignees(projectId);
    const currentBoardColumns = useKanbanStore((state) => state.columns);

    const [boards, setBoards] = React.useState<Board[]>([]);
    const [showForm, setShowForm] = React.useState(false);
    const [title, setTitle] = React.useState('');
    const [selectedBoardId, setSelectedBoardId] = React.useState(boardId ?? '');
    const [columns, setColumns] = React.useState<KanbanColumn[]>(currentBoardColumns);
    const [selectedColumnId, setSelectedColumnId] = React.useState(currentBoardColumns[0]?.id ?? '');
    const [priority, setPriority] = React.useState<TaskPriority>('medium' as TaskPriority);
    const [assigneeId, setAssigneeId] = React.useState('');
    const [dueDate, setDueDate] = React.useState('');
    const [isSaving, setIsSaving] = React.useState(false);
    const [formError, setFormError] = React.useState<string | null>(null);

    React.useEffect(() => { loadSubtasks(); }, [loadSubtasks]);
    React.useEffect(() => { loadAssignees(); }, [loadAssignees]);
    React.useEffect(() => { boardService.listBoards(projectId).then(setBoards); }, [projectId]);

    React.useEffect(() => {
        if (selectedBoardId === boardId) {
            setColumns(currentBoardColumns);
            setSelectedColumnId(currentBoardColumns[0]?.id ?? '');
            return;
        }
        if (!selectedBoardId) return;
        kanbanService.getBoard(projectId, selectedBoardId).then((board) => {
            setColumns(board.columns);
            setSelectedColumnId(board.columns[0]?.id ?? '');
        });
    }, [selectedBoardId, boardId, projectId, currentBoardColumns]);

    const isDoneColumn = (columnId: string) =>
        currentBoardColumns.find((c) => c.id === columnId)?.isDone ?? false;

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!title.trim() || !selectedColumnId) return;

        setIsSaving(true);
        setFormError(null);
        try {
            await addSubtask({
                title: title.trim(),
                description: '',
                priority,
                boardId: selectedBoardId,
                columnId: selectedColumnId,
                assigneeId: assigneeId || undefined,
                dueDate: dueDate || undefined,
            });
            setTitle('');
            setPriority('medium' as TaskPriority);
            setAssigneeId('');
            setDueDate('');
            setShowForm(false);
        } catch {
            setFormError('Failed to create subtask.');
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <div className="space-y-4">
            {error && (
                <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            {isLoading && subtasks.length === 0 ? (
                <p className="text-sm text-muted-foreground">Loading subtasks…</p>
            ) : subtasks.length === 0 ? (
                <div className="text-center text-muted-foreground py-8">
                    <ListTree className="mx-auto mb-2 opacity-40" size={28} />
                    <p className="text-sm">No subtasks yet</p>
                </div>
            ) : (
                <ul className="space-y-2">
                    {subtasks.map((subtask) => (
                        <li key={subtask.id}>
                            <button
                                type="button"
                                onClick={() => onOpenSubtask?.(subtask.id)}
                                className="w-full flex items-center justify-between gap-2 rounded-lg border border-border px-3 py-2 text-left hover:border-primary/30 hover:bg-muted/40 transition-colors"
                            >
                                <span className="truncate text-sm font-medium text-foreground">{subtask.title}</span>
                                <span className="flex items-center gap-2 shrink-0">
                                    {subtask.assignee && (
                                        <span className="flex items-center gap-1 text-xs text-muted-foreground">
                                            <User size={12} />
                                            {subtask.assignee.name}
                                        </span>
                                    )}
                                    <span className={cn(
                                        'text-xs font-medium px-2 py-0.5 rounded-full border',
                                        isDoneColumn(subtask.columnId)
                                            ? 'text-success bg-success/10 border-success/20'
                                            : 'text-muted-foreground bg-muted border-border'
                                    )}>
                                        {isDoneColumn(subtask.columnId) ? 'Done' : 'Not done'}
                                    </span>
                                </span>
                            </button>
                        </li>
                    ))}
                </ul>
            )}

            {showForm ? (
                <form onSubmit={handleSubmit} className="space-y-3 border border-dashed border-input rounded-lg p-3">
                    {formError && <p className="text-sm text-destructive">{formError}</p>}
                    <Input
                        label="Subtask title"
                        value={title}
                        onChange={setTitle}
                        placeholder="Enter subtask title..."
                        required
                    />
                    <div className="grid grid-cols-2 gap-3">
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
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                        <Select
                            label="Priority"
                            value={priority}
                            onChange={(value) => setPriority(value as TaskPriority)}
                            options={[
                                { value: 'low', label: 'Low' },
                                { value: 'medium', label: 'Medium' },
                                { value: 'high', label: 'High' },
                                { value: 'urgent', label: 'Urgent' },
                            ]}
                        />
                        <Select
                            label="Assignee"
                            value={assigneeId}
                            onChange={setAssigneeId}
                            options={[
                                { value: '', label: 'Unassigned' },
                                ...assignees.map((a) => ({ value: String(a.user_id), label: a.user_name })),
                            ]}
                        />
                    </div>
                    <Input
                        label="Due date"
                        type="date"
                        value={dueDate}
                        onChange={setDueDate}
                    />
                    <div className="flex gap-2">
                        <Button type="button" variant="outline" onClick={() => setShowForm(false)} className="flex-1">
                            Cancel
                        </Button>
                        <Button type="submit" loading={isSaving} disabled={!title.trim() || !selectedColumnId} className="flex-1">
                            Add Subtask
                        </Button>
                    </div>
                </form>
            ) : (
                <Button variant="outline" icon={Plus} onClick={() => setShowForm(true)} className="w-full justify-start">
                    Add Subtask
                </Button>
            )}
        </div>
    );
};
