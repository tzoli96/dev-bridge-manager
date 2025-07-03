'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { Icons } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

// Egyszerű típusok
interface Task {
    id: string;
    title: string;
    description: string;
    priority: 'low' | 'medium' | 'high' | 'urgent';
    assignee?: string;
    estimatedHours: number;
    loggedHours: number;
    comments: Comment[];
    dueDate?: string;
}

interface Comment {
    id: string;
    text: string;
    author: string;
    timestamp: string;
}

interface Column {
    id: string;
    title: string;
    color: string;
    tasks: Task[];
}

export default function BoardPage() {
    const [columns, setColumns] = useState<Column[]>([
        {
            id: 'todo',
            title: 'To Do',
            color: 'bg-blue-500',
            tasks: [
                {
                    id: 'task-1',
                    title: 'Setup project structure',
                    description: 'Create the basic project structure and configure development environment.',
                    priority: 'high',
                    assignee: 'John Doe',
                    estimatedHours: 4,
                    loggedHours: 1.5,
                    comments: [],
                    dueDate: '2024-12-20'
                },
                {
                    id: 'task-2',
                    title: 'Design user interface',
                    description: 'Create wireframes and mockups for the main user interface.',
                    priority: 'medium',
                    estimatedHours: 8,
                    loggedHours: 0,
                    comments: []
                }
            ]
        },
        {
            id: 'in-progress',
            title: 'In Progress',
            color: 'bg-yellow-500',
            tasks: [
                {
                    id: 'task-3',
                    title: 'Implement authentication',
                    description: 'Build user registration, login, and password reset functionality.',
                    priority: 'high',
                    assignee: 'Jane Smith',
                    estimatedHours: 12,
                    loggedHours: 7,
                    comments: [
                        {
                            id: 'comment-1',
                            text: 'Started working on JWT implementation',
                            author: 'Jane Smith',
                            timestamp: '2024-12-15T10:30:00Z'
                        }
                    ],
                    dueDate: '2024-12-22'
                }
            ]
        },
        {
            id: 'review',
            title: 'Review',
            color: 'bg-purple-500',
            tasks: [
                {
                    id: 'task-4',
                    title: 'Code review: API endpoints',
                    description: 'Review the new API endpoints for data validation and security.',
                    priority: 'medium',
                    assignee: 'Mike Johnson',
                    estimatedHours: 2,
                    loggedHours: 1,
                    comments: []
                }
            ]
        },
        {
            id: 'done',
            title: 'Done',
            color: 'bg-green-500',
            tasks: [
                {
                    id: 'task-5',
                    title: 'Database schema design',
                    description: 'Complete the database schema and migration files.',
                    priority: 'low',
                    assignee: 'John Doe',
                    estimatedHours: 6,
                    loggedHours: 6,
                    comments: [
                        {
                            id: 'comment-2',
                            text: 'Schema looks good, all constraints are in place.',
                            author: 'Jane Smith',
                            timestamp: '2024-12-14T15:20:00Z'
                        }
                    ]
                }
            ]
        }
    ]);

    const [draggedTask, setDraggedTask] = useState<Task | null>(null);
    const [draggedFrom, setDraggedFrom] = useState<string | null>(null);
    const [dragOverColumn, setDragOverColumn] = useState<string | null>(null);
    const [editingTask, setEditingTask] = useState<string | null>(null);
    const [showComments, setShowComments] = useState<string | null>(null);

    // Drag & Drop handlers
    const handleDragStart = (e: React.DragEvent, task: Task, fromColumnId: string) => {
        setDraggedTask(task);
        setDraggedFrom(fromColumnId);
        e.dataTransfer.effectAllowed = 'move';
        (e.target as HTMLElement).style.opacity = '0.5';
    };

    const handleDragEnd = (e: React.DragEvent) => {
        (e.target as HTMLElement).style.opacity = '1';
        setDraggedTask(null);
        setDraggedFrom(null);
        setDragOverColumn(null);
    };

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault();
    };

    const handleDragEnter = (e: React.DragEvent, columnId: string) => {
        e.preventDefault();
        if (draggedFrom !== columnId) {
            setDragOverColumn(columnId);
        }
    };

    const handleDragLeave = (e: React.DragEvent) => {
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        const x = e.clientX;
        const y = e.clientY;

        if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
            setDragOverColumn(null);
        }
    };

    const handleDrop = (e: React.DragEvent, toColumnId: string) => {
        e.preventDefault();
        setDragOverColumn(null);

        if (!draggedTask || !draggedFrom || draggedFrom === toColumnId) return;

        setColumns(prev => prev.map(col => {
            if (col.id === draggedFrom) {
                return { ...col, tasks: col.tasks.filter(t => t.id !== draggedTask.id) };
            }
            if (col.id === toColumnId) {
                return { ...col, tasks: [...col.tasks, draggedTask] };
            }
            return col;
        }));
    };

    // Priority colors
    const getPriorityColor = (priority: string) => {
        switch (priority) {
            case 'urgent': return 'border-l-red-500 bg-red-50';
            case 'high': return 'border-l-orange-500 bg-orange-50';
            case 'medium': return 'border-l-yellow-500 bg-yellow-50';
            case 'low': return 'border-l-green-500 bg-green-50';
            default: return 'border-l-gray-500 bg-gray-50';
        }
    };

    const formatDate = (dateString: string) => {
        const date = new Date(dateString);
        const now = new Date();
        const diffTime = date.getTime() - now.getTime();
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

        if (diffDays === 0) return 'Today';
        if (diffDays === 1) return 'Tomorrow';
        if (diffDays === -1) return 'Yesterday';
        if (diffDays < 0) return `${Math.abs(diffDays)} days ago`;
        return `${diffDays} days`;
    };

    return (
        <div className="min-h-screen bg-gray-50">
            {/* Header */}
            <div className="bg-white border-b border-gray-200 px-6 py-4">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900">Kanban Board</h1>
                        <p className="text-gray-600 mt-1">Manage your tasks with drag & drop</p>
                    </div>
                    <div className="flex items-center gap-3">
                        <Link
                            href="/"
                            className="text-blue-600 hover:text-blue-700 font-medium"
                        >
                            ← Back to Dashboard
                        </Link>
                        <Button>
                            <Icons.Plus />
                            New Task
                        </Button>
                    </div>
                </div>
            </div>

            {/* Board */}
            <div className="p-6 overflow-x-auto">
                <div className="flex gap-6 min-w-max">
                    {columns.map((column) => (
                        <div key={column.id} className="w-80 bg-white rounded-lg shadow-sm border border-gray-200">
                            {/* Column Header */}
                            <div className="p-4 border-b border-gray-200">
                                <div className="flex items-center justify-between mb-3">
                                    <div className="flex items-center gap-2">
                                        <div className={cn("w-3 h-3 rounded-full", column.color)} />
                                        <h3 className="font-semibold text-gray-900">{column.title}</h3>
                                    </div>
                                    <span className="bg-gray-100 text-gray-700 px-2 py-1 rounded-full text-sm font-medium">
                    {column.tasks.length}
                  </span>
                                </div>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="w-full justify-start border-2 border-dashed border-gray-300 hover:border-blue-400"
                                >
                                    <Icons.Plus />
                                    Add Task
                                </Button>
                            </div>

                            {/* Drop Zone */}
                            <div
                                className={cn(
                                    "p-4 min-h-[400px] space-y-3 transition-colors",
                                    dragOverColumn === column.id && "bg-blue-50 border-2 border-blue-300 border-dashed"
                                )}
                                onDragOver={handleDragOver}
                                onDragEnter={(e) => handleDragEnter(e, column.id)}
                                onDragLeave={handleDragLeave}
                                onDrop={(e) => handleDrop(e, column.id)}
                            >
                                {column.tasks.length === 0 ? (
                                    <div className="text-center text-gray-500 py-8">
                                        <div className="text-4xl mb-2">📋</div>
                                        <p className="text-sm">No tasks yet</p>
                                    </div>
                                ) : (
                                    column.tasks.map((task) => (
                                        <div
                                            key={task.id}
                                            draggable
                                            onDragStart={(e) => handleDragStart(e, task, column.id)}
                                            onDragEnd={handleDragEnd}
                                            className={cn(
                                                "group bg-white rounded-lg border-l-4 shadow-sm hover:shadow-md transition-all duration-200 cursor-move p-4",
                                                getPriorityColor(task.priority)
                                            )}
                                        >
                                            {/* Task Header */}
                                            <div className="flex items-start justify-between mb-3">
                                                <h4 className="font-medium text-gray-900 text-sm flex-1 pr-2">
                                                    {task.title}
                                                </h4>
                                                <Icons.Grip />
                                            </div>

                                            {/* Description */}
                                            <p className="text-sm text-gray-600 mb-3 line-clamp-2">
                                                {task.description}
                                            </p>

                                            {/* Progress */}
                                            {task.estimatedHours > 0 && (
                                                <div className="mb-3">
                                                    <div className="flex items-center justify-between text-xs text-gray-600 mb-1">
                                                        <span>Progress</span>
                                                        <span>{task.loggedHours}h / {task.estimatedHours}h</span>
                                                    </div>
                                                    <div className="w-full bg-gray-200 rounded-full h-2">
                                                        <div
                                                            className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                                                            style={{
                                                                width: `${Math.min((task.loggedHours / task.estimatedHours) * 100, 100)}%`
                                                            }}
                                                        />
                                                    </div>
                                                </div>
                                            )}

                                            {/* Meta Info */}
                                            <div className="flex items-center justify-between text-xs text-gray-500 mb-3">
                                                <div className="flex items-center gap-3">
                                                    {task.assignee && (
                                                        <div className="flex items-center gap-1">
                                                            <Icons.User />
                                                            <span>{task.assignee}</span>
                                                        </div>
                                                    )}
                                                    {task.dueDate && (
                                                        <div className="flex items-center gap-1">
                                                            <Icons.Calendar />
                                                            <span>{formatDate(task.dueDate)}</span>
                                                        </div>
                                                    )}
                                                </div>
                                                <div className="flex items-center gap-1">
                                                    <Icons.Clock />
                                                    <span>{task.loggedHours}h</span>
                                                </div>
                                            </div>

                                            {/* Actions */}
                                            <div className="flex items-center justify-between pt-3 border-t border-gray-100">
                                                <div className="flex items-center gap-1">
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        onClick={() => setEditingTask(task.id)}
                                                        className="opacity-0 group-hover:opacity-100 transition-opacity"
                                                    >
                                                        <Icons.Edit />
                                                    </Button>
                                                    <div className="relative">
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            onClick={() => setShowComments(task.id)}
                                                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                                                        >
                                                            <Icons.Message />
                                                        </Button>
                                                        {task.comments.length > 0 && (
                                                            <span className="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full w-4 h-4 flex items-center justify-center">
                                {task.comments.length}
                              </span>
                                                        )}
                                                    </div>
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="opacity-0 group-hover:opacity-100 transition-opacity"
                                                    >
                                                        <Icons.Timer />
                                                    </Button>
                                                </div>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="opacity-0 group-hover:opacity-100 transition-opacity text-red-600 hover:bg-red-50"
                                                >
                                                    <Icons.Trash />
                                                </Button>
                                            </div>
                                        </div>
                                    ))
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            </div>

            {/* Simple Comment Modal */}
            {showComments && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
                    <div className="bg-white rounded-lg max-w-md w-full p-6">
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="text-lg font-semibold">Comments</h3>
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setShowComments(null)}
                            >
                                <Icons.X />
                            </Button>
                        </div>
                        <div className="space-y-3 mb-4">
                            {columns
                                .flatMap(col => col.tasks)
                                .find(task => task.id === showComments)
                                ?.comments.map(comment => (
                                    <div key={comment.id} className="bg-gray-50 p-3 rounded">
                                        <div className="flex items-center justify-between mb-1">
                                            <span className="font-medium text-sm">{comment.author}</span>
                                            <span className="text-xs text-gray-500">
                      {new Date(comment.timestamp).toLocaleDateString()}
                    </span>
                                        </div>
                                        <p className="text-sm text-gray-700">{comment.text}</p>
                                    </div>
                                )) || (
                                <p className="text-gray-500 text-center py-4">No comments yet</p>
                            )}
                        </div>
                        <div className="flex gap-2">
                            <input
                                type="text"
                                placeholder="Add a comment..."
                                className="flex-1 p-2 border border-gray-300 rounded text-sm"
                            />
                            <Button size="sm">Send</Button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}