'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import { FileText, Paperclip, MessageSquare, History, ListTree } from 'lucide-react';
import { Modal } from '@/components/ui/modal';
import { Tabs, TabItem } from '@/components/ui/tabs';
import { useTasks } from '@/hooks/kanban';
import { useKanbanStore } from '@/stores/kanban';
import { taskService } from '@/services/kanban';
import { TaskPriority } from '@/types/kanban';
import { cn } from '@/lib/utils';
import { DescriptionTab } from './DescriptionTab';
import { AttachmentsTab } from './AttachmentsTab';
import { CommentsTab } from './CommentsTab';
import { HistoryTab } from './HistoryTab';
import { SubtasksTab } from './SubtasksTab';

export type TaskDetailTab = 'description' | 'attachments' | 'comments' | 'subtasks' | 'history';

const ALL_TABS: TabItem[] = [
    { id: 'description', label: 'Description', icon: FileText },
    { id: 'attachments', label: 'Attachments', icon: Paperclip },
    { id: 'comments', label: 'Comments', icon: MessageSquare },
    { id: 'subtasks', label: 'Subtasks', icon: ListTree },
    { id: 'history', label: 'History', icon: History },
];

const PRIORITY_BADGE: Record<TaskPriority, string> = {
    [TaskPriority.LOW]: 'text-success bg-success/10 border-success/20',
    [TaskPriority.MEDIUM]: 'text-warning bg-warning/10 border-warning/20',
    [TaskPriority.HIGH]: 'text-orange-700 bg-orange-100 border-orange-200 dark:text-orange-400 dark:bg-orange-950/40 dark:border-orange-900',
    [TaskPriority.URGENT]: 'text-destructive bg-destructive/10 border-destructive/20',
};

const PRIORITY_LABEL: Record<TaskPriority, string> = {
    [TaskPriority.LOW]: 'Low',
    [TaskPriority.MEDIUM]: 'Medium',
    [TaskPriority.HIGH]: 'High',
    [TaskPriority.URGENT]: 'Urgent',
};

interface TaskDetailModalProps {
    isOpen: boolean;
    taskId: string;
    initialTab: TaskDetailTab;
    onClose: () => void;
    onDeletePermanently?: () => Promise<void>;
    onNavigate?: (taskId: string) => void;
}

export const TaskDetailModal: React.FC<TaskDetailModalProps> = ({ isOpen, taskId, initialTab, onClose, onDeletePermanently, onNavigate }) => {
    const [activeTab, setActiveTab] = React.useState<TaskDetailTab>(initialTab);
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask } = useTasks(projectId);
    const addTaskToStore = useKanbanStore((state) => state.addTask);
    const task = getTask(taskId);

    React.useEffect(() => {
        if (isOpen) setActiveTab(initialTab);
    }, [isOpen, taskId, initialTab]);

    // A task opened by id (via a parent/subtask badge) may live on a board that was
    // never loaded into the store. GET /tasks/:taskId is board-independent, so fetch it
    // directly on a store miss and push it in; addTask() already no-ops the column-list
    // push when the task's columnId doesn't match any currently-loaded column.
    React.useEffect(() => {
        if (isOpen && taskId && !task) {
            taskService.getTask(projectId, taskId).then(addTaskToStore).catch(() => {});
        }
    }, [isOpen, taskId, task, projectId, addTaskToStore]);

    const tabs = ALL_TABS.filter((tab) => tab.id !== 'subtasks' || !task?.parentTask);

    return (
        <Modal
            isOpen={isOpen}
            onClose={onClose}
            title={
                <div className="flex items-center gap-2.5 min-w-0">
                    <span className="truncate">{task?.title ?? 'Task'}</span>
                    {task && (
                        <span className={cn('shrink-0 text-xs font-medium px-2 py-0.5 rounded-full border', PRIORITY_BADGE[task.priority])}>
                            {PRIORITY_LABEL[task.priority]}
                        </span>
                    )}
                </div>
            }
            size="xl"
        >
            {!task ? (
                <div className="p-6 text-sm text-muted-foreground">Loading…</div>
            ) : (
                <>
                    <Tabs tabs={tabs} activeTab={activeTab} onChange={(id) => setActiveTab(id as TaskDetailTab)} />
                    <div className="p-6">
                        {activeTab === 'description' && (
                            <DescriptionTab taskId={taskId} onDeletePermanently={onDeletePermanently} />
                        )}
                        {activeTab === 'attachments' && <AttachmentsTab taskId={taskId} />}
                        {activeTab === 'comments' && <CommentsTab taskId={taskId} />}
                        {activeTab === 'subtasks' && (
                            <SubtasksTab taskId={taskId} onOpenSubtask={onNavigate} />
                        )}
                        {activeTab === 'history' && <HistoryTab taskId={taskId} />}
                    </div>
                </>
            )}
        </Modal>
    );
};
