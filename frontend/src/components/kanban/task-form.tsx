'use client';

import React, { useState, useEffect, useRef } from 'react';
import { useParams } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useTasks, useAttachments } from '@/hooks/kanban';
import { useProject } from '@/hooks/projects/use-project';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useAuth } from '@/contexts/AuthContext';
import { AttachmentList } from './attachment-list';
import type { TaskFormData, TaskPriority, TagLevel } from '@/types/kanban';
import { X, Trash2, Paperclip } from 'lucide-react';

const LEVEL_ORDER: TagLevel[] = ['low', 'medium', 'high'];
const LEVEL_LABEL: Record<TagLevel, string> = { low: 'Low', medium: 'Medium', high: 'High' };
const LEVEL_CHIP_CLASS: Record<TagLevel, string> = {
    low: 'bg-primary/10 text-primary',
    medium: 'bg-primary/10 text-primary',
    high: 'bg-primary/20 text-primary font-semibold'
};

interface TaskFormProps {
    taskId?: string;
    columnId?: string;
    onSubmit: (data: any) => Promise<void>;
    onCancel: () => void;
    onDeletePermanently?: () => Promise<void>;
}

export const TaskForm: React.FC<TaskFormProps> = ({
                                                      taskId,
                                                      columnId,
                                                      onSubmit,
                                                      onCancel,
                                                      onDeletePermanently
                                                  }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask } = useTasks(projectId);
    const { uploadAttachments, deleteAttachment, isLoading: attachmentsLoading, error: attachmentsError } = useAttachments(projectId);
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const canManageAttachments = hasPermission('tasks:edit', projectId);
    const currentTask = taskId ? getTask(taskId) : undefined;
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);
    const [tagInput, setTagInput] = useState('');
    const [formData, setFormData] = useState<TaskFormData>({
        title: '',
        description: '',
        htmlDescription: '',
        priority: 'medium' as TaskPriority,
        assigneeId: '',
        estimatedHours: 0,
        tags: [],
        dueDate: ''
    });

    // Load existing task data if editing
    useEffect(() => {
        if (taskId) {
            const task = getTask(taskId);
            if (task) {
                setFormData({
                    title: task.title,
                    description: task.description,
                    htmlDescription: task.htmlDescription || task.description,
                    priority: task.priority,
                    assigneeId: task.assigneeId || '',
                    estimatedHours: task.estimatedHours,
                    tags: task.tags.map(tag => ({ name: tag.name, level: tag.level })),
                    dueDate: task.dueDate || ''
                });
            }
        }
    }, [taskId, getTask]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);

        try {
            await onSubmit(formData);
        } catch (error) {
            console.error('Error submitting task:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const handleFieldChange = (field: keyof TaskFormData, value: any) => {
        setFormData(prev => ({ ...prev, [field]: value }));
    };

    const handleAddTag = (raw: string) => {
        const value = raw.trim();
        if (!value) return;
        if (formData.tags.some(t => t.name.toLowerCase() === value.toLowerCase())) {
            setTagInput('');
            return;
        }
        handleFieldChange('tags', [...formData.tags, { name: value, level: 'medium' as TagLevel }]);
        setTagInput('');
    };

    const handleRemoveTag = (name: string) => {
        handleFieldChange('tags', formData.tags.filter(t => t.name !== name));
    };

    const handleCycleTagLevel = (name: string) => {
        handleFieldChange('tags', formData.tags.map(t => {
            if (t.name !== name) return t;
            const next = LEVEL_ORDER[(LEVEL_ORDER.indexOf(t.level) + 1) % LEVEL_ORDER.length];
            return { ...t, level: next };
        }));
    };

    const handleTagInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault();
            handleAddTag(tagInput);
        } else if (e.key === 'Backspace' && tagInput === '' && formData.tags.length > 0) {
            handleRemoveTag(formData.tags[formData.tags.length - 1].name);
        }
    };

    const handleAttachmentFiles = async (fileList: FileList | null) => {
        if (!fileList || fileList.length === 0 || !taskId) return;
        try {
            await uploadAttachments(taskId, Array.from(fileList));
        } catch (error) {
            console.error('Error uploading attachments:', error);
        }
        if (fileInputRef.current) fileInputRef.current.value = '';
    };

    const handleAttachmentDrop = (e: React.DragEvent) => {
        e.preventDefault();
        handleAttachmentFiles(e.dataTransfer.files);
    };

    const handleDeletePermanently = async () => {
        if (!onDeletePermanently) return;
        const taskForWarning = taskId ? getTask(taskId) : undefined;
        const subtaskWarning = taskForWarning?.subtaskProgress?.total
            ? ` This will also permanently delete its ${taskForWarning.subtaskProgress.total} subtask${taskForWarning.subtaskProgress.total === 1 ? '' : 's'}.`
            : '';
        if (!confirm(`Delete this task permanently? It will be removed from every board it appears on.${subtaskWarning}`)) return;

        setIsDeleting(true);
        try {
            await onDeletePermanently();
        } finally {
            setIsDeleting(false);
        }
    };

    return (
        <form onSubmit={handleSubmit} className="p-6 space-y-6">
            {/* Title */}
            <Input
                label="Task Title"
                value={formData.title}
                onChange={(value) => handleFieldChange('title', value)}
                placeholder="Enter task title..."
                required
            />

            {/* Description */}
            <div className="space-y-2">
                <label className="block text-sm font-medium text-foreground">
                    Description
                </label>
                <RichTextEditor
                    content={formData.htmlDescription}
                    onChange={(html, text) => {
                        handleFieldChange('htmlDescription', html);
                        handleFieldChange('description', text);
                    }}
                    placeholder="Enter task description..."
                    minHeight="120px"
                />
            </div>

            {/* Priority and Estimate */}
            <div className="grid grid-cols-2 gap-4">
                <Select
                    label="Priority"
                    value={formData.priority}
                    onChange={(value) => handleFieldChange('priority', value)}
                    options={[
                        { value: 'low', label: 'Low' },
                        { value: 'medium', label: 'Medium' },
                        { value: 'high', label: 'High' },
                        { value: 'urgent', label: 'Urgent' }
                    ]}
                />

                <Input
                    label="Estimated Hours"
                    type="number"
                    value={formData.estimatedHours}
                    onChange={(value) => handleFieldChange('estimatedHours', parseFloat(value) || 0)}
                    placeholder="0"
                    min="0"
                    step="0.5"
                />
            </div>

            {/* Due Date */}
            <Input
                label="Due Date"
                type="date"
                value={formData.dueDate}
                onChange={(value) => handleFieldChange('dueDate', value)}
            />

            {/* Tags */}
            <div className="space-y-2">
                <label className="block text-sm font-medium text-foreground">
                    Tags
                </label>
                <div className="flex flex-wrap items-center gap-2 rounded-md border border-input p-2 focus-within:ring-2 focus-within:ring-ring focus-within:border-ring">
                    {formData.tags.map((tag) => (
                        <Badge key={tag.name} variant="secondary" className={`gap-1 ${LEVEL_CHIP_CLASS[tag.level]}`}>
                            <button
                                type="button"
                                onClick={() => handleCycleTagLevel(tag.name)}
                                className="flex items-center gap-0.5"
                                aria-label={`${tag.name} importance: ${LEVEL_LABEL[tag.level]} — click to change`}
                                title={`Importance: ${LEVEL_LABEL[tag.level]}`}
                            >
                                {LEVEL_ORDER.map((lvl, i) => (
                                    <span
                                        key={lvl}
                                        className={`h-1 w-1 rounded-full ${i <= LEVEL_ORDER.indexOf(tag.level) ? 'bg-current' : 'bg-current opacity-25'}`}
                                    />
                                ))}
                            </button>
                            {tag.name}
                            <button
                                type="button"
                                onClick={() => handleRemoveTag(tag.name)}
                                className="ml-1 hover:text-destructive"
                                aria-label={`Remove tag ${tag.name}`}
                            >
                                <X size={12} />
                            </button>
                        </Badge>
                    ))}
                    <input
                        type="text"
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        onKeyDown={handleTagInputKeyDown}
                        onBlur={() => handleAddTag(tagInput)}
                        placeholder={formData.tags.length === 0 ? 'Type a tag and press Enter...' : ''}
                        className="flex-1 min-w-[120px] border-none outline-none text-sm py-1"
                    />
                </div>
            </div>

            {/* Attachments */}
            {taskId && (
                <div className="space-y-2">
                    <label className="block text-sm font-medium text-foreground">
                        Attachments
                    </label>
                    <AttachmentList
                        attachments={currentTask?.attachments ?? []}
                        currentUserId={user ? String(user.id) : undefined}
                        canManage={canManageAttachments}
                        onDelete={(attachmentId) => deleteAttachment(taskId, attachmentId)}
                    />
                    <div
                        onDragOver={(e) => e.preventDefault()}
                        onDrop={handleAttachmentDrop}
                        className="rounded-md border-2 border-dashed border-input p-4 text-center text-sm text-muted-foreground"
                    >
                        <input
                            ref={fileInputRef}
                            type="file"
                            multiple
                            className="hidden"
                            disabled={attachmentsLoading}
                            onChange={(e) => handleAttachmentFiles(e.target.files)}
                        />
                        <Button type="button" variant="outline" icon={Paperclip} disabled={attachmentsLoading} onClick={() => fileInputRef.current?.click()}>
                            Add files
                        </Button>
                        <p className="mt-1">or drag and drop (max 5 files, 10MB each)</p>
                    </div>
                    {attachmentsError && (
                        <p className="text-sm text-destructive">{attachmentsError}</p>
                    )}
                </div>
            )}

            {/* Actions */}
            <div className="space-y-3 pt-4">
                {taskId && onDeletePermanently && (
                    <Button
                        type="button"
                        variant="ghost"
                        onClick={handleDeletePermanently}
                        loading={isDeleting}
                        icon={Trash2}
                        className="text-destructive hover:bg-destructive/10"
                    >
                        Delete permanently
                    </Button>
                )}

                <div className="flex space-x-3">
                    <Button
                        type="button"
                        variant="outline"
                        onClick={onCancel}
                        className="flex-1"
                    >
                        Cancel
                    </Button>
                    <Button
                        type="submit"
                        loading={isLoading}
                        className="flex-1"
                    >
                        {taskId ? 'Update Task' : 'Create Task'}
                    </Button>
                </div>
            </div>
        </form>
    );
};
