'use client';

import React, { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useTasks } from '@/hooks/kanban';
import { useTaskAssignees } from '@/hooks/kanban/use-task-assignees';
import type { TagLevel, TaskPriority, UpdateTaskData } from '@/types/kanban';
import { Check, Loader2, AlertCircle, Trash2, X } from 'lucide-react';

type FieldStatus = 'idle' | 'saving' | 'saved' | 'error';

const LEVEL_ORDER: TagLevel[] = ['low', 'medium', 'high'];

interface DescriptionTabProps {
    taskId: string;
    onDeletePermanently?: () => Promise<void>;
}

export const DescriptionTab: React.FC<DescriptionTabProps> = ({ taskId, onDeletePermanently }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask, updateTask } = useTasks(projectId);
    const { assignees, loadAssignees } = useTaskAssignees(projectId);
    const task = getTask(taskId);

    const [title, setTitle] = useState(task?.title ?? '');
    const [description, setDescription] = useState(task?.description ?? '');
    const [htmlDescription, setHtmlDescription] = useState(task?.htmlDescription ?? '');
    const [priority, setPriority] = useState<TaskPriority>(task?.priority ?? ('medium' as TaskPriority));
    const [dueDate, setDueDate] = useState(task?.dueDate ?? '');
    const [estimatedHours, setEstimatedHours] = useState(task?.estimatedHours ?? 0);
    const [assigneeId, setAssigneeId] = useState(task?.assigneeId ?? '');
    const [tags, setTags] = useState(task?.tags ?? []);
    const [tagInput, setTagInput] = useState('');
    const [status, setStatus] = useState<Record<string, FieldStatus>>({});
    const [isDeleting, setIsDeleting] = useState(false);

    useEffect(() => { loadAssignees(); }, [loadAssignees]);

    const save = async (field: string, data: UpdateTaskData) => {
        setStatus((s) => ({ ...s, [field]: 'saving' }));
        try {
            await updateTask(taskId, data);
            setStatus((s) => ({ ...s, [field]: 'saved' }));
            setTimeout(() => setStatus((s) => ({ ...s, [field]: 'idle' })), 1500);
        } catch {
            setStatus((s) => ({ ...s, [field]: 'error' }));
        }
    };

    const Indicator = ({ field }: { field: string }) => {
        const s = status[field] ?? 'idle';
        if (s === 'saving') return <Loader2 className="w-3.5 h-3.5 animate-spin text-gray-400" />;
        if (s === 'saved') return <Check className="w-3.5 h-3.5 text-green-600" />;
        if (s === 'error') return <AlertCircle className="w-3.5 h-3.5 text-red-600" />;
        return null;
    };

    const commitTags = (next: typeof tags) => {
        setTags(next);
        save('tags', { tags: next.map((t) => ({ name: t.name, level: t.level })) });
    };
    const handleAddTag = () => {
        if (!tagInput.trim()) return;
        commitTags([...tags, { id: tagInput, name: tagInput, color: 'blue', level: 'medium' as TagLevel }]);
        setTagInput('');
    };
    const handleRemoveTag = (name: string) => commitTags(tags.filter((t) => t.name !== name));
    const handleCycleTagLevel = (name: string) => commitTags(tags.map((t) =>
        t.name === name ? { ...t, level: LEVEL_ORDER[(LEVEL_ORDER.indexOf(t.level) + 1) % LEVEL_ORDER.length] } : t
    ));

    if (!task) return null;

    const handleTitleBlur = () => {
        if (!title.trim()) {
            setTitle(task.title);
            setStatus((s) => ({ ...s, title: 'error' }));
            return;
        }
        save('title', { title });
    };

    return (
        <div className="space-y-6">
            <div className="flex items-center gap-2">
                <div className="flex-1">
                    <Input
                        label="Task Title"
                        value={title}
                        onChange={setTitle}
                        onBlur={handleTitleBlur}
                        required
                    />
                </div>
                <Indicator field="title" />
            </div>

            <div className="space-y-2">
                <label className="block text-sm font-medium text-gray-700">Description</label>
                <div className="flex items-start gap-2">
                    <div className="flex-1">
                        <RichTextEditor
                            content={htmlDescription}
                            onChange={(html, text) => { setHtmlDescription(html); setDescription(text); }}
                            onBlur={() => save('description', { description, htmlDescription })}
                            minHeight="120px"
                        />
                    </div>
                    <Indicator field="description" />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Select
                            label="Priority"
                            value={priority}
                            onChange={(value) => {
                                const next = value as TaskPriority;
                                setPriority(next);
                                save('priority', { priority: next });
                            }}
                            options={[
                                { value: 'low', label: 'Low' },
                                { value: 'medium', label: 'Medium' },
                                { value: 'high', label: 'High' },
                                { value: 'urgent', label: 'Urgent' },
                            ]}
                        />
                    </div>
                    <Indicator field="priority" />
                </div>
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Input
                            label="Estimated Hours"
                            type="number"
                            min="0"
                            step="0.5"
                            value={String(estimatedHours)}
                            onChange={(v) => setEstimatedHours(Number(v))}
                            onBlur={() => save('estimatedHours', { estimatedHours })}
                        />
                    </div>
                    <Indicator field="estimatedHours" />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Input
                            label="Due Date"
                            type="date"
                            value={dueDate}
                            onChange={(v) => { setDueDate(v); save('dueDate', { dueDate: v }); }}
                        />
                    </div>
                    <Indicator field="dueDate" />
                </div>
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Select
                            label="Assignee"
                            value={assigneeId}
                            onChange={(value) => { setAssigneeId(value); save('assignee', { assigneeId: value }); }}
                            options={[
                                { value: '', label: 'Unassigned' },
                                ...assignees.map((a) => ({ value: String(a.user_id), label: a.user_name })),
                            ]}
                        />
                    </div>
                    <Indicator field="assignee" />
                </div>
            </div>

            <div className="space-y-2">
                <label className="block text-sm font-medium text-gray-700">Tags</label>
                <div className="flex flex-wrap items-center gap-2 p-2 border rounded-lg border-gray-300 focus-within:ring-blue-500 focus-within:border-blue-500">
                    {tags.map((tag) => (
                        <Badge key={tag.name} variant="secondary" className="gap-1">
                            <button type="button" onClick={() => handleCycleTagLevel(tag.name)}>{tag.level}</button>
                            {tag.name}
                            <button type="button" onClick={() => handleRemoveTag(tag.name)} className="ml-1 hover:text-red-600">
                                <X size={12} />
                            </button>
                        </Badge>
                    ))}
                    <input
                        className="flex-1 min-w-[120px] outline-none py-1"
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleAddTag(); } }}
                        placeholder="Add tag…"
                    />
                </div>
            </div>

            {onDeletePermanently && (
                <div className="pt-4 border-t">
                    <button
                        type="button"
                        onClick={async () => {
                            if (!confirm('Delete this task permanently? It will be removed from every board it appears on.')) return;
                            setIsDeleting(true);
                            try { await onDeletePermanently(); } finally { setIsDeleting(false); }
                        }}
                        disabled={isDeleting}
                        className="flex items-center gap-2 text-sm text-red-600 hover:text-red-700"
                    >
                        <Trash2 size={14} /> {isDeleting ? 'Deleting…' : 'Delete permanently'}
                    </button>
                </div>
            )}
        </div>
    );
};
