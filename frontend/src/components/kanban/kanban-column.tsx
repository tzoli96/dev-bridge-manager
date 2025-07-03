'use client';

import React from 'react';
import { TaskCard } from './task-card';
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
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
    onDeleteTask: (taskId: string) => void;
    onOpenComments: (taskId: string) => void;
    onOpenTimeLog: (taskId: string) => void;
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
                                                              onDeleteTask,
                                                              onOpenComments,
                                                              onOpenTimeLog,
                                                              onDragStart,
                                                              onDragEnd
                                                          }) => {
    const isWipLimitReached = column.maxTasks && tasks.length >= column.maxTasks;
    const canAddTask = permissions.canCreateTasks && !isWipLimitReached;

    return (
        <div className="w-80 bg-gray-50 rounded-lg flex flex-col">
            {/* Column Header */}
            <div className="p-4 border-b border-gray-200">
                <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                        <div className={cn("w-3 h-3 rounded-full", column.color)} />
                        <h3 className="font-semibold text-gray-900">{column.title}</h3>
                    </div>

                    <div className="flex items-center gap-2">
            <span className={cn(
                "px-2 py-1 rounded-full text-xs font-medium",
                isWipLimitReached
                    ? "bg-red-100 text-red-800"
                    : "bg-gray-200 text-gray-700"
            )}>
              {column.maxTasks ? `${tasks.length}/${column.maxTasks}` : tasks.length}
            </span>
                    </div>
                </div>

                {canAddTask && (
                    <Button
                        onClick={onAddTask}
                        variant="ghost"
                        size="sm"
                        icon={Plus}
                        className="w-full justify-start border-2 border-dashed border-gray-300 hover:border-blue-400"
                    >
                        Add Task
                    </Button>
                )}

                {isWipLimitReached && (
                    <div className="text-xs text-red-600 mt-2">
                        WIP limit reached
                    </div>
                )}
            </div>

            {/* Drop Zone */}
            <div
                className={cn(
                    "flex-1 p-4 space-y-3 min-h-[400px] transition-colors",
                    isHighlighted && "bg-blue-50 border-2 border-blue-300 border-dashed"
                )}
                onDragOver={dragHandlers.onDragOver}
                onDragEnter={dragHandlers.onDragEnter}
                onDragLeave={dragHandlers.onDragLeave}
                onDrop={dragHandlers.onDrop}
            >
                {tasks.length === 0 ? (
                    <div className="text-center text-gray-500 py-8">
                        <div className="text-4xl mb-2">📋</div>
                        <p className="text-sm">No tasks yet</p>
                    </div>
                ) : (
                    tasks.map((task) => (
                        <TaskCard
                            key={task.id}
                            task={task}
                            permissions={permissions}
                            onEdit={() => onEditTask(task.id)}
                            onDelete={() => onDeleteTask(task.id)}
                            onOpenComments={() => onOpenComments(task.id)}
                            onOpenTimeLog={() => onOpenTimeLog(task.id)}
                            onDragStart={() => onDragStart(task)}
                            onDragEnd={onDragEnd}
                        />
                    ))
                )}
            </div>
        </div>
    );
};