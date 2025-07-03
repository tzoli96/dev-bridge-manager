'use client';

import React, { createContext, useContext, useEffect } from 'react';
import { useKanbanStore } from '@/stores/kanban/kanban.store';
import { kanbanService, taskService, commentService, timeEntryService } from '@/services/kanban';
import type { KanbanBoard } from '@/types/kanban';

interface KanbanContextValue {
    projectId: string;
    isLoading: boolean;
    error: string | null;
}

const KanbanContext = createContext<KanbanContextValue | null>(null);

interface KanbanProviderProps {
    projectId: string;
    children: React.ReactNode;
}

export const KanbanProvider: React.FC<KanbanProviderProps> = ({
                                                                  projectId,
                                                                  children
                                                              }) => {
    const {
        setBoard,
        setTasks,
        setComments,
        setTimeEntries,
        setLoading,
        setError,
        isLoading,
        error
    } = useKanbanStore();

    // Initial data loading
    useEffect(() => {
        const loadKanbanData = async () => {
            if (!projectId) return;

            setLoading(true);
            setError(null);

            try {
                // Load board data
                const [board, tasks, comments, timeEntries] = await Promise.all([
                    kanbanService.getBoard(projectId).catch(() => createDefaultBoard(projectId)),
                    taskService.getTasks(projectId).catch(() => []),
                    // commentService.getComments(projectId).catch(() => []),
                    // timeEntryService.getProjectTimeEntries(projectId).catch(() => [])
                ]);

                setBoard(board);
                setTasks(tasks);
                // setComments(comments);
                // setTimeEntries(timeEntries);
            } catch (err) {
                console.error('Error loading kanban data:', err);
                setError(err instanceof Error ? err.message : 'Failed to load board data');

                // Create default board on error
                const defaultBoard = createDefaultBoard(projectId);
                setBoard(defaultBoard);
            } finally {
                setLoading(false);
            }
        };

        loadKanbanData();
    }, [projectId, setBoard, setTasks, setComments, setTimeEntries, setLoading, setError]);

    const contextValue: KanbanContextValue = {
        projectId,
        isLoading,
        error
    };

    return (
        <KanbanContext.Provider value={contextValue}>
            {children}
        </KanbanContext.Provider>
    );
};

export const useKanbanContext = (): KanbanContextValue => {
    const context = useContext(KanbanContext);
    if (!context) {
        throw new Error('useKanbanContext must be used within KanbanProvider');
    }
    return context;
};

// Helper function to create default board
const createDefaultBoard = (projectId: string): KanbanBoard => {
    return {
        id: `board-${projectId}`,
        projectId,
        columns: [
            {
                id: 'todo',
                title: 'To Do',
                color: 'bg-blue-500',
                position: 0,
                tasks: [
                    {
                        id: 'task-1',
                        title: 'Setup project structure',
                        description: 'Create the basic project structure and configure development environment.',
                        htmlDescription: '<p>Create the basic project structure and configure development environment.</p>',
                        priority: 'high' as any,
                        status: 'todo' as any,
                        columnId: 'todo',
                        projectId,
                        assigneeId: '',
                        estimatedHours: 4,
                        loggedHours: 1.5,
                        tags: [
                            { id: 'tag-1', name: 'setup', color: '#3B82F6' },
                            { id: 'tag-2', name: 'urgent', color: '#EF4444' }
                        ],
                        timeEntries: [],
                        comments: [],
                        attachments: [],
                        position: 0,
                        createdAt: new Date().toISOString(),
                        updatedAt: new Date().toISOString(),
                        createdBy: 'demo-user',
                        updatedBy: 'demo-user'
                    },
                    {
                        id: 'task-2',
                        title: 'Design user interface',
                        description: 'Create wireframes and mockups for the main user interface.',
                        htmlDescription: '<p>Create <strong>wireframes</strong> and <em>mockups</em> for the main user interface.</p>',
                        priority: 'medium' as any,
                        status: 'todo' as any,
                        columnId: 'todo',
                        projectId,
                        assigneeId: '',
                        estimatedHours: 8,
                        loggedHours: 0,
                        tags: [
                            { id: 'tag-3', name: 'design', color: '#8B5CF6' },
                            { id: 'tag-4', name: 'ui', color: '#06B6D4' }
                        ],
                        timeEntries: [],
                        comments: [],
                        attachments: [],
                        position: 1,
                        createdAt: new Date().toISOString(),
                        updatedAt: new Date().toISOString(),
                        createdBy: 'demo-user',
                        updatedBy: 'demo-user'
                    }
                ],
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            },
            {
                id: 'in-progress',
                title: 'In Progress',
                color: 'bg-yellow-500',
                position: 1,
                maxTasks: 3,
                tasks: [
                    {
                        id: 'task-3',
                        title: 'Implement authentication',
                        description: 'Build user registration, login, and password reset functionality.',
                        htmlDescription: '<p>Build user <strong>registration</strong>, <strong>login</strong>, and password reset functionality.</p><ul><li>JWT tokens</li><li>Password hashing</li><li>Email verification</li></ul>',
                        priority: 'high' as any,
                        status: 'in_progress' as any,
                        columnId: 'in-progress',
                        projectId,
                        assigneeId: 'user-1',
                        assignee: {
                            id: 'user-1',
                            name: 'John Doe',
                            email: 'john@example.com',
                            avatar: ''
                        },
                        estimatedHours: 12,
                        loggedHours: 7,
                        tags: [
                            { id: 'tag-5', name: 'backend', color: '#10B981' },
                            { id: 'tag-6', name: 'security', color: '#F59E0B' }
                        ],
                        timeEntries: [],
                        comments: [
                            {
                                id: 'comment-1',
                                taskId: 'task-3',
                                content: 'Started working on JWT implementation',
                                htmlContent: '<p>Started working on <strong>JWT implementation</strong></p>',
                                userId: 'user-1',
                                user: {
                                    id: 'user-1',
                                    name: 'John Doe',
                                    avatar: ''
                                },
                                isEdited: false,
                                createdAt: new Date(Date.now() - 86400000).toISOString(),
                                updatedAt: new Date(Date.now() - 86400000).toISOString()
                            }
                        ],
                        attachments: [],
                        position: 0,
                        dueDate: new Date(Date.now() + 7 * 86400000).toISOString(),
                        createdAt: new Date().toISOString(),
                        updatedAt: new Date().toISOString(),
                        createdBy: 'demo-user',
                        updatedBy: 'demo-user'
                    }
                ],
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            },
            {
                id: 'review',
                title: 'Review',
                color: 'bg-purple-500',
                position: 2,
                tasks: [
                    {
                        id: 'task-4',
                        title: 'Code review: API endpoints',
                        description: 'Review the new API endpoints for data validation and security.',
                        htmlDescription: '<p>Review the new API endpoints for <em>data validation</em> and <em>security</em>.</p>',
                        priority: 'medium' as any,
                        status: 'review' as any,
                        columnId: 'review',
                        projectId,
                        assigneeId: 'user-2',
                        assignee: {
                            id: 'user-2',
                            name: 'Jane Smith',
                            email: 'jane@example.com',
                            avatar: ''
                        },
                        estimatedHours: 2,
                        loggedHours: 1,
                        tags: [
                            { id: 'tag-7', name: 'review', color: '#8B5CF6' },
                            { id: 'tag-8', name: 'api', color: '#06B6D4' }
                        ],
                        timeEntries: [],
                        comments: [],
                        attachments: [],
                        position: 0,
                        createdAt: new Date().toISOString(),
                        updatedAt: new Date().toISOString(),
                        createdBy: 'demo-user',
                        updatedBy: 'demo-user'
                    }
                ],
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            },
            {
                id: 'done',
                title: 'Done',
                color: 'bg-green-500',
                position: 3,
                tasks: [
                    {
                        id: 'task-5',
                        title: 'Database schema design',
                        description: 'Complete the database schema and migration files.',
                        htmlDescription: '<p>Complete the database schema and migration files.</p><blockquote>All tables and relationships are now defined.</blockquote>',
                        priority: 'low' as any,
                        status: 'done' as any,
                        columnId: 'done',
                        projectId,
                        assigneeId: 'user-1',
                        assignee: {
                            id: 'user-1',
                            name: 'John Doe',
                            email: 'john@example.com',
                            avatar: ''
                        },
                        estimatedHours: 6,
                        loggedHours: 6,
                        tags: [
                            { id: 'tag-9', name: 'database', color: '#10B981' },
                            { id: 'tag-10', name: 'completed', color: '#6B7280' }
                        ],
                        timeEntries: [
                            {
                                id: 'time-1',
                                taskId: 'task-5',
                                hours: 3,
                                description: 'Initial schema design',
                                date: new Date(Date.now() - 2 * 86400000).toISOString().split('T')[0],
                                userId: 'user-1',
                                user: {
                                    id: 'user-1',
                                    name: 'John Doe',
                                    avatar: ''
                                },
                                createdAt: new Date(Date.now() - 2 * 86400000).toISOString(),
                                updatedAt: new Date(Date.now() - 2 * 86400000).toISOString()
                            },
                            {
                                id: 'time-2',
                                taskId: 'task-5',
                                hours: 3,
                                description: 'Migration files and testing',
                                date: new Date(Date.now() - 86400000).toISOString().split('T')[0],
                                userId: 'user-1',
                                user: {
                                    id: 'user-1',
                                    name: 'John Doe',
                                    avatar: ''
                                },
                                createdAt: new Date(Date.now() - 86400000).toISOString(),
                                updatedAt: new Date(Date.now() - 86400000).toISOString()
                            }
                        ],
                        comments: [
                            {
                                id: 'comment-2',
                                taskId: 'task-5',
                                content: 'Schema looks good, all constraints are in place.',
                                htmlContent: '<p>Schema looks good, all constraints are in place. ✅</p>',
                                userId: 'user-2',
                                user: {
                                    id: 'user-2',
                                    name: 'Jane Smith',
                                    avatar: ''
                                },
                                isEdited: false,
                                createdAt: new Date(Date.now() - 3600000).toISOString(),
                                updatedAt: new Date(Date.now() - 3600000).toISOString()
                            }
                        ],
                        attachments: [],
                        position: 0,
                        createdAt: new Date().toISOString(),
                        updatedAt: new Date().toISOString(),
                        createdBy: 'demo-user',
                        updatedBy: 'demo-user'
                    }
                ],
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            }
        ],
        settings: {
            enableWipLimits: true,
            enableTimeTracking: true,
            enableComments: true,
            enablePriorities: true,
            enableTags: true,
            defaultEstimateUnit: 'hours'
        },
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString()
    };
};