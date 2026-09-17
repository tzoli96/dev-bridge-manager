'use client';

import React, { useEffect, useState } from 'react';
import { attachmentService } from '@/services/kanban';
import { Button } from '@/components/ui/button';
import { Download, FileText, Trash2 } from 'lucide-react';
import type { TaskAttachment } from '@/types/kanban';

interface AttachmentListProps {
    attachments: TaskAttachment[];
    currentUserId?: string;
    canManage: boolean;
    onDelete: (attachmentId: string) => void | Promise<void>;
}

const isImage = (mimeType: string) => mimeType.startsWith('image/');

const formatSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const AttachmentThumbnail: React.FC<{ attachment: TaskAttachment }> = ({ attachment }) => {
    const [blobUrl, setBlobUrl] = useState<string | null>(null);

    useEffect(() => {
        if (!isImage(attachment.mimeType)) return;
        let objectUrl: string | null = null;
        let cancelled = false;

        attachmentService.downloadAttachment(attachment.downloadUrl).then((blob) => {
            if (cancelled) return;
            objectUrl = URL.createObjectURL(blob);
            setBlobUrl(objectUrl);
        }).catch(() => {});

        return () => {
            cancelled = true;
            if (objectUrl) URL.revokeObjectURL(objectUrl);
        };
    }, [attachment.downloadUrl, attachment.mimeType]);

    if (isImage(attachment.mimeType)) {
        return blobUrl
            ? <img src={blobUrl} alt={attachment.originalName} className="w-10 h-10 rounded object-cover" />
            : <div className="w-10 h-10 rounded bg-muted animate-pulse" />;
    }

    return (
        <div className="w-10 h-10 rounded bg-muted flex items-center justify-center">
            <FileText className="w-5 h-5 text-muted-foreground" />
        </div>
    );
};

export const AttachmentList: React.FC<AttachmentListProps> = ({ attachments, currentUserId, canManage, onDelete }) => {
    const handleDownload = async (attachment: TaskAttachment) => {
        try {
            const blob = await attachmentService.downloadAttachment(attachment.downloadUrl);
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = attachment.originalName;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            URL.revokeObjectURL(url);
        } catch (error) {
            console.error('Error downloading attachment:', error);
        }
    };

    if (attachments.length === 0) return null;

    return (
        <div className="space-y-2">
            {attachments.map((attachment) => {
                const canDelete = canManage || attachment.uploadedById === currentUserId;
                return (
                    <div key={attachment.id} className="flex items-center gap-3 rounded-lg border border-border p-2">
                        <AttachmentThumbnail attachment={attachment} />
                        <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{attachment.originalName}</p>
                            <p className="text-xs text-muted-foreground">{formatSize(attachment.size)}</p>
                        </div>
                        <Button type="button" variant="ghost" size="sm" onClick={() => handleDownload(attachment)} icon={Download} />
                        {canDelete && (
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => onDelete(attachment.id)}
                                icon={Trash2}
                                className="text-destructive hover:bg-destructive/10"
                            />
                        )}
                    </div>
                );
            })}
        </div>
    );
};
