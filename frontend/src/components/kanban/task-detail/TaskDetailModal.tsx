'use client';

import React from 'react';
import { Modal } from '@/components/ui/modal';
import { Tabs, TabItem } from '@/components/ui/tabs';
import { DescriptionTab } from './DescriptionTab';
import { AttachmentsTab } from './AttachmentsTab';
import { CommentsTab } from './CommentsTab';
import { HistoryTab } from './HistoryTab';

export type TaskDetailTab = 'description' | 'attachments' | 'comments' | 'history';

const TABS: TabItem[] = [
    { id: 'description', label: 'Description' },
    { id: 'attachments', label: 'Attachments' },
    { id: 'comments', label: 'Comments' },
    { id: 'history', label: 'History' },
];

interface TaskDetailModalProps {
    isOpen: boolean;
    taskId: string;
    initialTab: TaskDetailTab;
    onClose: () => void;
    onDeletePermanently?: () => Promise<void>;
}

export const TaskDetailModal: React.FC<TaskDetailModalProps> = ({ isOpen, taskId, initialTab, onClose, onDeletePermanently }) => {
    const [activeTab, setActiveTab] = React.useState<TaskDetailTab>(initialTab);

    React.useEffect(() => {
        if (isOpen) setActiveTab(initialTab);
    }, [isOpen, taskId, initialTab]);

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Task" size="xl">
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
