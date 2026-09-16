// components/kanban/task-card.tsx
'use client';

import React from 'react';
import {
    Edit2,
    MessageSquare,
    Timer,
    Trash2,
    FolderPlus,
    GripVertical,
    Clock,
    Calendar,
    AlertCircle,
    ListTree
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ProgressBar } from '@/components/ui/progress-bar';
import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';
import { hu } from 'date-fns/locale';
import type { Task, KanbanPermissions, TaskPriority, TagLevel } from '@/types/kanban';
import { cn } from '@/lib/utils';

const TAG_LEVEL_ALPHA: Record<TagLevel, string> = { low: '15', medium: '28', high: '45' };

interface TaskCardProps {
    task: Task;
    permissions: KanbanPermissions;
    onEdit: () => void;
    onRemove: () => void;
    onAddToBoard: () => void;
    onOpenComments: () => void;
    onOpenTimeLog: () => void;
    onOpenParentTask?: () => void;
    onDragStart: () => void;
    onDragEnd: () => void;
}

const priorityIcons: Record<TaskPriority, React.ComponentType<any>> = {
    low: Clock,
    medium: Clock,
    high: AlertCircle,
    urgent: AlertCircle
};

const priorityIconColors: Record<TaskPriority, string> = {
    low: 'text-success',
    medium: 'text-warning',
    high: 'text-orange-600 dark:text-orange-400',
    urgent: 'text-destructive'
};

export const TaskCard: React.FC<TaskCardProps> = ({
                                                      task,
                                                      permissions,
                                                      onEdit,
                                                      onRemove,
                                                      onAddToBoard,
                                                      onOpenComments,
                                                      onOpenTimeLog,
                                                      onOpenParentTask,
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
                "group bg-card rounded-lg border border-border hover:border-primary/30 hover:shadow-sm transition-all duration-150 p-3.5",
                permissions.canMoveTasks ? "cursor-move" : "cursor-default"
            )}
        >
            {/* Header */}
            <div className="flex items-start justify-between gap-2 mb-2">
                <div className="flex items-start gap-2 flex-1 min-w-0">
                    <PriorityIcon
                        size={14}
                        className={cn("mt-0.5 flex-shrink-0", priorityIconColors[task.priority])}
                    />
                    <h4 className="font-medium text-foreground line-clamp-2 text-sm leading-snug">
                        {task.title}
                    </h4>
                </div>

                {permissions.canMoveTasks && (
                    <GripVertical
                        size={14}
                        className="text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
                    />
                )}
            </div>

            {task.parentTask && (
                <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); onOpenParentTask?.(); }}
                    className="mb-2 text-xs text-muted-foreground hover:text-primary truncate text-left block"
                >
                    ↳ {task.parentTask.title}
                </button>
            )}

            <div className="space-y-2">
                {/* Tags */}
                {task.tags.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                        {task.tags.slice(0, 3).map((tag) => (
                            <Badge
                                key={tag.id}
                                variant="secondary"
                                className={cn("text-xs", tag.level === 'high' && "font-semibold")}
                                style={{ backgroundColor: tag.color + TAG_LEVEL_ALPHA[tag.level], color: tag.color }}
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
                    <div>
                        <div className="flex items-center justify-between text-xs text-muted-foreground mb-1">
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
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <div className="flex items-center gap-3">
                        {task.assignee && (
                            <div className="flex items-center gap-1.5">
                                <div className="flex items-center justify-center w-5 h-5 rounded-full bg-primary/15 text-primary text-[10px] font-bold flex-shrink-0">
                                    {task.assignee.name.slice(0, 2).toUpperCase()}
                                </div>
                                <span className="truncate max-w-20">{task.assignee.name}</span>
                            </div>
                        )}

                        {task.dueDate && (
                            <div className={cn(
                                "flex items-center gap-1",
                                isOverdue && "text-destructive font-medium"
                            )}>
                                {isOverdue ? <AlertCircle size={12} /> : <Calendar size={12} />}
                                <span className="truncate">
                {formatDistanceToNow(new Date(task.dueDate), {
                    addSuffix: true,
                    locale: hu
                })}
              </span>
                            </div>
                        )}

                        {task.comments.length > 0 && (
                            <div className="flex items-center gap-1">
                                <MessageSquare size={12} />
                                <span>{task.comments.length}</span>
                            </div>
                        )}

                        {task.subtaskProgress && (
                            <div className="flex items-center gap-1">
                                <ListTree size={12} />
                                <span>{task.subtaskProgress.done}/{task.subtaskProgress.total}</span>
                            </div>
                        )}
                    </div>

                    <div className="flex items-center gap-1 flex-shrink-0">
                        <Clock size={12} />
                        <span>{task.loggedHours}h</span>
                    </div>
                </div>
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between mt-3 pt-3 border-t border-border">
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
                    <div className="flex items-center gap-1">
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                onAddToBoard();
                            }}
                            icon={FolderPlus}
                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                        />
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                const subtaskWarning = task.subtaskProgress?.total
                                    ? ` This will also permanently delete its ${task.subtaskProgress.total} subtask${task.subtaskProgress.total === 1 ? '' : 's'}.`
                                    : '';
                                if (confirm(`Remove this task from this board?${subtaskWarning}`)) {
                                    onRemove();
                                }
                            }}
                            icon={Trash2}
                            className="opacity-0 group-hover:opacity-100 transition-opacity text-destructive hover:bg-destructive/10"
                        />
                    </div>
                )}
            </div>
        </div>
    );
};