'use client';

import React, { useState, useRef } from 'react';
import { useParams } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useComments, useAttachments } from '@/hooks/kanban';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useAuth } from '@/contexts/AuthContext';
import { AttachmentList } from './attachment-list';
import { formatDistanceToNow } from 'date-fns';
import { hu } from 'date-fns/locale';
import { MessageSquare, Send, Edit2, Trash2, Paperclip } from 'lucide-react';

interface CommentSectionProps {
    taskId: string;
}

export const CommentSection: React.FC<CommentSectionProps> = ({ taskId }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getCommentsByTask, addComment, updateComment, deleteComment, isLoading } = useComments(projectId);
    const { uploadAttachments, deleteAttachment, isLoading: attachmentsLoading, error: attachmentsError } = useAttachments(projectId);
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const canManageAttachments = hasPermission('tasks:edit', projectId);
    const [newComment, setNewComment] = useState('');
    const [newCommentHtml, setNewCommentHtml] = useState('');
    const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
    const [pendingFiles, setPendingFiles] = useState<File[]>([]);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const comments = getCommentsByTask(taskId);

    const handleAddComment = async () => {
        if (!newComment.trim()) return;

        try {
            const comment = await addComment(taskId, {
                content: newComment.trim(),
                htmlContent: newCommentHtml || newComment.trim()
            });
            if (pendingFiles.length > 0) {
                await uploadAttachments(taskId, pendingFiles, comment.id);
                setPendingFiles([]);
            }
            setNewComment('');
            setNewCommentHtml('');
        } catch (error) {
            console.error('Error adding comment:', error);
        }
    };

    const handleUpdateComment = async (commentId: string, content: string, htmlContent: string) => {
        try {
            await updateComment(commentId, { content, htmlContent });
            setEditingCommentId(null);
        } catch (error) {
            console.error('Error updating comment:', error);
        }
    };

    const handleDeleteComment = async (commentId: string) => {
        if (!confirm('Are you sure you want to delete this comment?')) return;

        try {
            await deleteComment(commentId);
        } catch (error) {
            console.error('Error deleting comment:', error);
        }
    };

    return (
        <div className="p-6 space-y-6">
            {/* Comments List */}
            <div className="space-y-4 max-h-96 overflow-y-auto">
                {comments.length === 0 ? (
                    <div className="text-center py-8 text-muted-foreground">
                        <MessageSquare className="w-12 h-12 mx-auto mb-3 text-muted-foreground" />
                        <p>No comments yet.</p>
                        <p className="text-sm">Be the first to add a comment!</p>
                    </div>
                ) : (
                    comments.map((comment) => (
                        <div key={comment.id} className="bg-muted rounded-lg p-4">
                            <div className="flex items-start justify-between mb-2">
                                <div className="flex items-center gap-2">
                                    <div className="w-8 h-8 bg-primary rounded-full flex items-center justify-center">
                    <span className="text-white text-sm font-medium">
                      {comment.user?.name.charAt(0).toUpperCase() || 'U'}
                    </span>
                                    </div>
                                    <div>
                                        <p className="font-medium text-sm">{comment.user?.name || 'Unknown User'}</p>
                                        <p className="text-xs text-muted-foreground">
                                            {formatDistanceToNow(new Date(comment.createdAt), {
                                                addSuffix: true,
                                                locale: hu
                                            })}
                                            {comment.isEdited && ' (edited)'}
                                        </p>
                                    </div>
                                </div>

                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => setEditingCommentId(comment.id)}
                                        icon={Edit2}
                                    />
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => handleDeleteComment(comment.id)}
                                        icon={Trash2}
                                        className="text-destructive hover:bg-destructive/10"
                                    />
                                </div>
                            </div>

                            <div className="prose prose-sm max-w-none">
                                <div dangerouslySetInnerHTML={{
                                    __html: comment.htmlContent || comment.content
                                }} />
                            </div>

                            <AttachmentList
                                attachments={comment.attachments}
                                currentUserId={user ? String(user.id) : undefined}
                                canManage={canManageAttachments}
                                onDelete={(attachmentId) => deleteAttachment(taskId, attachmentId, comment.id)}
                            />
                        </div>
                    ))
                )}
            </div>

            {/* Add Comment Form */}
            <div className="border-t pt-4">
                <div className="space-y-3">
                    <RichTextEditor
                        content={newCommentHtml}
                        onChange={(html, text) => {
                            setNewCommentHtml(html);
                            setNewComment(text);
                        }}
                        placeholder="Write a comment..."
                        minHeight="100px"
                    />

                    <div className="flex items-center gap-2">
                        <input
                            ref={fileInputRef}
                            type="file"
                            multiple
                            className="hidden"
                            disabled={attachmentsLoading}
                            onChange={(e) => setPendingFiles(Array.from(e.target.files ?? []))}
                        />
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            icon={Paperclip}
                            disabled={attachmentsLoading}
                            onClick={() => fileInputRef.current?.click()}
                        >
                            {pendingFiles.length > 0 ? `${pendingFiles.length} file(s) selected` : 'Attach files'}
                        </Button>
                    </div>
                    {attachmentsError && (
                        <p className="text-sm text-destructive">{attachmentsError}</p>
                    )}

                    <div className="flex justify-end">
                        <Button
                            onClick={handleAddComment}
                            disabled={!newComment.trim() || isLoading}
                            loading={isLoading}
                            icon={Send}
                        >
                            Add Comment
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
};