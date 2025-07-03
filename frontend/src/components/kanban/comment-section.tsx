'use client';

import React, { useState } from 'react';
import { Button } from '@/components/ui/button';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useComments } from '@/hooks/kanban';
import { formatDistanceToNow } from 'date-fns';
import { hu } from 'date-fns/locale';
import { MessageSquare, Send, Edit2, Trash2 } from 'lucide-react';

interface CommentSectionProps {
    taskId: string;
}

export const CommentSection: React.FC<CommentSectionProps> = ({ taskId }) => {
    const { getCommentsByTask, addComment, updateComment, deleteComment, isLoading } = useComments('');
    const [newComment, setNewComment] = useState('');
    const [newCommentHtml, setNewCommentHtml] = useState('');
    const [editingCommentId, setEditingCommentId] = useState<string | null>(null);

    const comments = getCommentsByTask(taskId);

    const handleAddComment = async () => {
        if (!newComment.trim()) return;

        try {
            await addComment(taskId, {
                content: newComment.trim(),
                htmlContent: newCommentHtml || newComment.trim()
            });
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
                    <div className="text-center py-8 text-gray-500">
                        <MessageSquare className="w-12 h-12 mx-auto mb-3 text-gray-300" />
                        <p>No comments yet.</p>
                        <p className="text-sm">Be the first to add a comment!</p>
                    </div>
                ) : (
                    comments.map((comment) => (
                        <div key={comment.id} className="bg-gray-50 rounded-lg p-4">
                            <div className="flex items-start justify-between mb-2">
                                <div className="flex items-center gap-2">
                                    <div className="w-8 h-8 bg-blue-500 rounded-full flex items-center justify-center">
                    <span className="text-white text-sm font-medium">
                      {comment.user?.name.charAt(0).toUpperCase() || 'U'}
                    </span>
                                    </div>
                                    <div>
                                        <p className="font-medium text-sm">{comment.user?.name || 'Unknown User'}</p>
                                        <p className="text-xs text-gray-500">
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
                                        className="text-red-600 hover:bg-red-50"
                                    />
                                </div>
                            </div>

                            <div className="prose prose-sm max-w-none">
                                <div dangerouslySetInnerHTML={{
                                    __html: comment.htmlContent || comment.content
                                }} />
                            </div>
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