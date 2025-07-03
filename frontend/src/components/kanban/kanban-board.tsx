'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import { KanbanColumn } from './kanban-column';
import { TaskForm } from './task-form';
import { CommentSection } from './comment-section';
import { TimeTracker } from './time-tracker';
import { Modal } from '@/components/ui/modal';
import { Button } from '@/components/ui/button';
import { useKanban, useTasks, useDragDrop } from '@/hooks/kanban';
import { ModalType } from '@/types/kanban';
import { Plus, Settings } from 'lucide-react';

/**
 * Fő Kanban Board komponens
 * Single Responsibility: Kanban board koordinálása és layout
 */
export const KanbanBoard: React.FC = () => {
    const { projectId } = useParams<{ projectId: string }>();

    // Hook-ok
    const { columns, isLoading, error, permissions } = useKanban(projectId);
    const { getTasksByColumn, createTask, updateTask, deleteTask, moveTask } = useTasks(projectId);
    const dragDrop = useDragDrop();

    // Modal state
    const [activeModal, setActiveModal] = React.useState<{
        type: ModalType | null;
        taskId?: string;
        columnId?: string;
    }>({ type: null });

    // Event handlers
    const handleAddTask = (columnId: string) => {
        if (!permissions.canCreateTasks) return;
        setActiveModal({ type: ModalType.TASK_EDIT, columnId });
    };

    const handleEditTask = (taskId: string) => {
        if (!permissions.canEditTasks) return;
        setActiveModal({ type: ModalType.TASK_EDIT, taskId });
    };

    const handleOpenComments = (taskId: string) => {
        setActiveModal({ type: ModalType.COMMENTS, taskId });
    };

    const handleOpenTimeLog = (taskId: string) => {
        if (!permissions.canViewTimeTracking) return;
        setActiveModal({ type: ModalType.TIME_LOG, taskId });
    };

    const handleTaskMove = async (result: any) => {
        if (!permissions.canMoveTasks) return;

        try {
            await moveTask(result.taskId, {
                columnId: result.toColumnId,
                position: result.newPosition || 0
            });
        } catch (error) {
            console.error('Failed to move task:', error);
        }
    };

    const closeModal = () => {
        setActiveModal({ type: null });
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="text-center text-red-600 p-4">
                <p>Error loading kanban board: {error}</p>
                <Button onClick={() => window.location.reload()} className="mt-2">
                    Retry
                </Button>
            </div>
        );
    }

    return (
        <div className="h-full flex flex-col">
            {/* Header */}
            <div className="flex items-center justify-between p-6 border-b">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">Kanban Board</h1>
                    <p className="text-gray-600 mt-1">Manage your project tasks</p>
                </div>

                <div className="flex items-center gap-2">
                    {permissions.canManageColumns && (
                        <Button variant="ghost" icon={Settings}>
                            Settings
                        </Button>
                    )}
                </div>
            </div>

            {/* Board */}
            <div className="flex-1 overflow-x-auto">
                <div className="flex gap-6 p-6 min-w-max">
                    {columns.map((column) => (
                        <KanbanColumn
                            key={column.id}
                            column={column}
                            tasks={getTasksByColumn(column.id)}
                            permissions={permissions}
                            isHighlighted={dragDrop.isColumnHighlighted(column.id)}
                            dragHandlers={{
                                onDragOver: dragDrop.handleDragOver,
                                onDragEnter: (e) => dragDrop.handleDragEnter(e, column.id),
                                onDragLeave: dragDrop.handleDragLeave,
                                onDrop: (e) => {
                                    const result = dragDrop.handleDrop(e, column.id);
                                    if (result) handleTaskMove(result);
                                }
                            }}
                            onAddTask={() => handleAddTask(column.id)}
                            onEditTask={handleEditTask}
                            onDeleteTask={deleteTask}
                            onOpenComments={handleOpenComments}
                            onOpenTimeLog={handleOpenTimeLog}
                            onDragStart={(task) => dragDrop.startDrag(task, column.id)}
                            onDragEnd={dragDrop.endDrag}
                        />
                    ))}
                </div>
            </div>

            {/* Modals */}
            <Modal
                isOpen={activeModal.type === ModalType.TASK_EDIT}
                onClose={closeModal}
                title={activeModal.taskId ? 'Edit Task' : 'Create Task'}
                size="lg"
            >
                <TaskForm
                    taskId={activeModal.taskId}
                    columnId={activeModal.columnId}
                    onSubmit={async (data) => {
                        if (activeModal.taskId) {
                            await updateTask(activeModal.taskId, data);
                        } else if (activeModal.columnId) {
                            await createTask({ ...data, columnId: activeModal.columnId });
                        }
                        closeModal();
                    }}
                    onCancel={closeModal}
                />
            </Modal>

            <Modal
                isOpen={activeModal.type === ModalType.COMMENTS}
                onClose={closeModal}
                title="Comments"
                size="lg"
            >
                {activeModal.taskId && (
                    <CommentSection taskId={activeModal.taskId} />
                )}
            </Modal>

            <Modal
                isOpen={activeModal.type === ModalType.TIME_LOG}
                onClose={closeModal}
                title="Time Tracking"
                size="lg"
            >
                {activeModal.taskId && (
                    <TimeTracker taskId={activeModal.taskId} />
                )}
            </Modal>
        </div>
    );
};
