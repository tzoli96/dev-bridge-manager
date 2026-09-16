'use client';

import React from 'react';
import { TaskCard } from './task-card';
import { Button } from '@/components/ui/button';
import { Inbox, Plus } from 'lucide-react';
import type {
    KanbanColumn as Column,
    Task,
    KanbanPermissions
} from '@/types/kanban';
import { cn } from '@/lib/utils';

interface KanbanColumnProps {
    column: Column;
    tasks: Task[];
    permissions: KanbanPermissions;
    isHighlighted: boolean;
    dragHandlers: {
        onDragOver: (e: React.DragEvent) => void;
        onDragEnter: (e: React.DragEvent) => void;
        onDragLeave: (e: React.DragEvent) => void;
        onDrop: (e: React.DragEvent) => void;
    };
    onAddTask: () => void;
    onEditTask: (taskId: string) => void;
    onRemoveTask: (taskId: string) => void;
    onAddToBoard: (taskId: string) => void;
    onOpenComments: (taskId: string) => void;
    onOpenTimeLog: (taskId: string) => void;
    onOpenParentTask: (taskId: string) => void;
    onDragStart: (task: Task) => void;
    onDragEnd: () => void;
}

export const KanbanColumn: React.FC<KanbanColumnProps> = ({
                                                              column,
                                                              tasks,
                                                              permissions,
                                                              isHighlighted,
                                                              dragHandlers,
                                                              onAddTask,
                                                              onEditTask,
                                                              onRemoveTask,
                                                              onAddToBoard,
                                                              onOpenComments,
                                                              onOpenTimeLog,
                                                              onOpenParentTask,
                                                              onDragStart,
                                                              onDragEnd
                                                          }) => {
    const isWipLimitReached = column.maxTasks && tasks.length >= column.maxTasks;
    const canAddTask = permissions.canCreateTasks && !isWipLimitReached;

    return (
        <div className="w-80 flex-shrink-0 flex flex-col">
            {/* Column Header */}
            <div className="flex items-center justify-between px-1 mb-3">
                <div className="flex items-center gap-2">
                    <div className={cn("w-2 h-2 rounded-full", column.color)} />
                    <h3 className="text-sm font-medium text-foreground">{column.title}</h3>
                    <span className={cn(
                        "text-xs tabular-nums",
                        isWipLimitReached ? "text-destructive font-medium" : "text-muted-foreground"
                    )}>
                        {column.maxTasks ? `${tasks.length}/${column.maxTasks}` : tasks.length}
                    </span>
                </div>
            </div>

            {/* Drop Zone */}
            <div
                className={cn(
                    "flex-1 rounded-xl bg-muted/40 p-2 space-y-2 min-h-[400px] transition-colors",
                    isHighlighted && "bg-primary/10 ring-2 ring-primary/30 ring-inset"
                )}
                onDragOver={dragHandlers.onDragOver}
                onDragEnter={dragHandlers.onDragEnter}
                onDragLeave={dragHandlers.onDragLeave}
                onDrop={dragHandlers.onDrop}
            >
                {canAddTask && (
                    <Button
                        onClick={onAddTask}
                        variant="ghost"
                        size="sm"
                        icon={Plus}
                        className="w-full justify-start text-muted-foreground hover:text-foreground"
                    >
                        Add task
                    </Button>
                )}

                {isWipLimitReached && (
                    <div className="text-xs text-destructive px-2">
                        WIP limit reached
                    </div>
                )}

                {tasks.length === 0 ? (
                    <div className="text-center text-muted-foreground py-8">
                        <Inbox className="mx-auto mb-2 opacity-40" size={28} />
                        <p className="text-sm">No tasks yet</p>
                    </div>
                ) : (
                    tasks.map((task) => (
                        <TaskCard
                            key={task.id}
                            task={task}
                            permissions={permissions}
                            onEdit={() => onEditTask(task.id)}
                            onRemove={() => onRemoveTask(task.id)}
                            onAddToBoard={() => onAddToBoard(task.id)}
                            onOpenComments={() => onOpenComments(task.id)}
                            onOpenTimeLog={() => onOpenTimeLog(task.id)}
                            onOpenParentTask={() => task.parentTask && onOpenParentTask(task.parentTask.id)}
                            onDragStart={() => onDragStart(task)}
                            onDragEnd={onDragEnd}
                        />
                    ))
                )}
            </div>
        </div>
    );
};