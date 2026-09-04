'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import { FileText, Paperclip, MessageSquare, History } from 'lucide-react';
import { Modal } from '@/components/ui/modal';
import { Tabs, TabItem } from '@/components/ui/tabs';
import { useTasks } from '@/hooks/kanban';
import { TaskPriority } from '@/types/kanban';
import { cn } from '@/lib/utils';
import { DescriptionTab } from './DescriptionTab';
import { AttachmentsTab } from './AttachmentsTab';
import { CommentsTab } from './CommentsTab';
import { HistoryTab } from './HistoryTab';

export type TaskDetailTab = 'description' | 'attachments' | 'comments' | 'history';

const TABS: TabItem[] = [
    { id: 'description', label: 'Description', icon: FileText },
    { id: 'attachments', label: 'Attachments', icon: Paperclip },
    { id: 'comments', label: 'Comments', icon: MessageSquare },
    { id: 'history', label: 'History', icon: History },
];

const PRIORITY_BADGE: Record<TaskPriority, string> = {
    [TaskPriority.LOW]: 'text-green-700 bg-green-100 border-green-200',
    [TaskPriority.MEDIUM]: 'text-yellow-700 bg-yellow-100 border-yellow-200',
    [TaskPriority.HIGH]: 'text-orange-700 bg-orange-100 border-orange-200',
    [TaskPriority.URGENT]: 'text-red-700 bg-red-100 border-red-200',
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
}

export const TaskDetailModal: React.FC<TaskDetailModalProps> = ({ isOpen, taskId, initialTab, onClose, onDeletePermanently }) => {
    const [activeTab, setActiveTab] = React.useState<TaskDetailTab>(initialTab);
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask } = useTasks(projectId);
    const task = getTask(taskId);

    React.useEffect(() => {
        if (isOpen) setActiveTab(initialTab);
    }, [isOpen, taskId, initialTab]);

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
            <Tabs tabs={TABS} activeTab={activeTab} onChange={(id) => setActiveTab(id as TaskDetailTab)} />
            <div className="p-6">
                {activeTab === 'description' && (
                    <DescriptionTab taskId={taskId} onDeletePermanently={onDeletePermanently} />
                )}
                {activeTab === 'attachments' && <AttachmentsTab taskId={taskId} />}
                {activeTab === 'comments' && <CommentsTab taskId={taskId} />}
                {activeTab === 'history' && <HistoryTab taskId={taskId} />}
            </div>
        </Modal>
    );
};
