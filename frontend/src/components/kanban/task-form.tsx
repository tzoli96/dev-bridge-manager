'use client';

import React, { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useTasks } from '@/hooks/kanban';
import { useProject } from '@/hooks/projects/use-project';
import type { TaskFormData, TaskPriority } from '@/types/kanban';
import { Save, X } from 'lucide-react';

interface TaskFormProps {
    taskId?: string;
    columnId?: string;
    onSubmit: (data: any) => Promise<void>;
    onCancel: () => void;
}

export const TaskForm: React.FC<TaskFormProps> = ({
                                                      taskId,
                                                      columnId,
                                                      onSubmit,
                                                      onCancel
                                                  }) => {
    const { getTask } = useTasks(''); // projectId will be from context
    const [isLoading, setIsLoading] = useState(false);
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
                    tags: task.tags.map(tag => tag.name),
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
                <label className="block text-sm font-medium text-gray-700">
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

            {/* Actions */}
            <div className="flex items-center justify-end gap-3 pt-4 border-t">
                <Button
                    type="button"
                    variant="ghost"
                    onClick={onCancel}
                    icon={X}
                >
                    Cancel
                </Button>
                <Button
                    type="submit"
                    loading={isLoading}
                    icon={Save}
                >
                    {taskId ? 'Update Task' : 'Create Task'}
                </Button>
            </div>
        </form>
    );
};
