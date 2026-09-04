'use client';

import React, { useRef } from 'react';
import { useParams } from 'next/navigation';
import { useAttachments, useTasks } from '@/hooks/kanban';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useAuth } from '@/contexts/AuthContext';
import { AttachmentList } from '../attachment-list';

interface AttachmentsTabProps {
    taskId: string;
}

export const AttachmentsTab: React.FC<AttachmentsTabProps> = ({ taskId }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask } = useTasks(projectId);
    const currentTask = getTask(taskId);
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const canManageAttachments = hasPermission('tasks:edit', projectId);
    const { uploadAttachments, deleteAttachment, isLoading, error } = useAttachments(projectId);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const handleFiles = async (files: FileList | null) => {
        if (!files || files.length === 0) return;
        await uploadAttachments(taskId, Array.from(files));
    };

    return (
        <div className="space-y-3">
            <AttachmentList
                attachments={currentTask?.attachments ?? []}
                currentUserId={user ? String(user.id) : undefined}
                canManage={canManageAttachments}
                onDelete={(attachmentId) => deleteAttachment(taskId, attachmentId)}
            />
            {error && <p className="text-sm text-red-600">{error}</p>}
            <div
                className="border-2 border-dashed rounded-lg p-4 text-center text-gray-500"
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => { e.preventDefault(); handleFiles(e.dataTransfer.files); }}
            >
                <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    className="hidden"
                    onChange={(e) => handleFiles(e.target.files)}
                />
                <button type="button" onClick={() => fileInputRef.current?.click()} disabled={isLoading}>
                    {isLoading ? 'Uploading…' : 'Drop files here or click to upload (max 10MB each)'}
                </button>
            </div>
        </div>
    );
};
