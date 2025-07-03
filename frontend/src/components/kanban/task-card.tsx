// components/kanban/task-card.tsx
'use client';

import React from 'react';
import {
    Edit2,
    MessageSquare,
    Timer,
    Trash2,
    GripVertical,
    User,
    Clock,
    Calendar,
    AlertCircle
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ProgressBar } from '@/components/ui/progress-bar';
import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';
import { hu } from 'date-fns/locale';
import type { Task, KanbanPermissions, TaskPriority } from '@/types/kanban';
import { cn } from '@/lib/utils';

interface TaskCardProps {
    task: Task;
    permissions: KanbanPermissions;
    onEdit: () => void;
    onDelete: () => void;
    onOpenComments: () => void;
    onOpenTimeLog: () => void;
    onDragStart: () => void;
    onDragEnd: () => void;
}

const priorityColors: Record<TaskPriority, string> = {
    low: 'border-l-green-500 bg-green-50',
    medium: 'border-l-yellow-500 bg-yellow-50',
    high: 'border-l-orange-500 bg-orange-50',
    urgent: 'border-l-red-500 bg-red-50'
};

const priorityIcons: Record<TaskPriority, React.ComponentType<any>> = {
    low: Clock,
    medium: Clock,
    high: AlertCircle,
    urgent: AlertCircle
};

const priorityIconColors: Record<TaskPriority, string> = {
    low: 'text-green-600',
    medium: 'text-yellow-600',
    high: 'text-orange-600',
    urgent: 'text-red-600'
};

export const TaskCard: React.FC<TaskCardProps> = ({
                                                      task,
                                                      permissions,
                                                      onEdit,
                                                      onDelete,
                                                      onOpenComments,
                                                      onOpenTimeLog,
                                                      onDragStart,
                                                      onDragEnd
                                                  }) => {
    const PriorityIcon = priorityIcons[task.priority];
    const progressPercentage = task.estimatedHours > 0
        ? (task.loggedHours / task.estimatedHours) * 100
        : 0;

    const isOverdue = task.dueDate && new Date(task.dueDate) < new Date();

    const handleDragStart = (e: React.DragEvent) => {
        if (!permissions.canMoveTasks) {
            e.preventDefault();
            return;
        }

        console.log('🎯 TaskCard drag start:', task.title);

        // Adatok tárolása a drag event-ben
        e.dataTransfer.setData('application/json', JSON.stringify({
            taskId: task.id,
            fromColumn: task.columnId
        }));
        e.dataTransfer.effectAllowed = 'move';

        // Visual feedback
        e.currentTarget.style.opacity = '0.5';

        // Hook callback hívása
        onDragStart();
    };

    const handleDragEnd = (e: React.DragEvent) => {
        console.log('🏁 TaskCard drag end:', task.title);

        // Reset visual feedback
        e.currentTarget.style.opacity = '1';

        // Hook callback hívása
        onDragEnd();
    };

    return (
        <div
            draggable={permissions.canMoveTasks}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
            className={cn(
                "group bg-white rounded-lg border-l-4 shadow-sm hover:shadow-md transition-all duration-200 p-4",
                priorityColors[task.priority],
                permissions.canMoveTasks ? "cursor-move" : "cursor-default"
            )}
        >
            {/* Header */}
            <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-2 flex-1">
                    <PriorityIcon
                        size={16}
                        className={priorityIconColors[task.priority]}
                    />
                    <h4 className="font-medium text-gray-900 line-clamp-2 text-sm">
                        {task.title}
                    </h4>
                </div>

                {permissions.canMoveTasks && (
                    <GripVertical
                        size={16}
                        className="text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
                    />
                )}
            </div>

            {/* Description */}
            {task.description && (
                <div className="mb-3">
                    <div
                        className="text-sm text-gray-600 line-clamp-3 prose prose-sm max-w-none"
                        dangerouslySetInnerHTML={{
                            __html: task.htmlDescription || task.description
                        }}
                    />
                </div>
            )}

            {/* Tags */}
            {task.tags.length > 0 && (
                <div className="flex flex-wrap gap-1 mb-3">
                    {task.tags.slice(0, 3).map((tag) => (
                        <Badge
                            key={tag.id}
                            variant="secondary"
                            className="text-xs"
                            style={{ backgroundColor: tag.color + '20', color: tag.color }}
                        >
                            {tag.name}
                        </Badge>
                    ))}
                    {task.tags.length > 3 && (
                        <Badge variant="secondary" className="text-xs">
                            +{task.tags.length - 3}
                        </Badge>
                    )}
                </div>
            )}

            {/* Progress Bar */}
            {task.estimatedHours > 0 && (
                <div className="mb-3">
                    <div className="flex items-center justify-between text-xs text-gray-600 mb-1">
                        <span>Progress</span>
                        <span>{task.loggedHours}h / {task.estimatedHours}h</span>
                    </div>
                    <ProgressBar
                        percentage={progressPercentage}
                        size="sm"
                        variant={progressPercentage > 100 ? 'danger' : 'default'}
                    />
                </div>
            )}

            {/* Meta Info */}
            <div className="flex items-center justify-between text-xs text-gray-500 mb-3">
                <div className="flex items-center gap-3">
                    {task.assignee && (
                        <div className="flex items-center gap-1">
                            <User size={12} />
                            <span className="truncate max-w-20">{task.assignee.name}</span>
                        </div>
                    )}

                    {task.dueDate && (
                        <div className={cn(
                            "flex items-center gap-1",
                            isOverdue && "text-red-600"
                        )}>
                            <Calendar size={12} />
                            <span className="truncate">
                {formatDistanceToNow(new Date(task.dueDate), {
                    addSuffix: true,
                    locale: hu
                })}
              </span>
                        </div>
                    )}
                </div>

                <div className="flex items-center gap-1 flex-shrink-0">
                    <Clock size={12} />
                    <span>{task.loggedHours}h</span>
                </div>
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between pt-3 border-t border-gray-100">
                <div className="flex items-center gap-1">
                    {permissions.canEditTasks && (
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                onEdit();
                            }}
                            icon={Edit2}
                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                        />
                    )}

                    <div className="relative">
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                onOpenComments();
                            }}
                            icon={MessageSquare}
                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                        />
                        {task.comments.length > 0 && (
                            <span className="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full w-4 h-4 flex items-center justify-center">
                {task.comments.length}
              </span>
                        )}
                    </div>

                    {permissions.canViewTimeTracking && (
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                onOpenTimeLog();
                            }}
                            icon={Timer}
                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                        />
                    )}
                </div>

                {permissions.canDeleteTasks && (
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={(e) => {
                            e.stopPropagation();
                            if (confirm('Are you sure you want to delete this task?')) {
                                onDelete();
                            }
                        }}
                        icon={Trash2}
                        className="opacity-0 group-hover:opacity-100 transition-opacity text-red-600 hover:bg-red-50"
                    />
                )}
            </div>
        </div>
    );
};