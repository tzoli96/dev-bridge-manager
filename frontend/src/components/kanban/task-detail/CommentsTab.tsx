'use client';

import React from 'react';
import { CommentSection } from '../comment-section';

interface CommentsTabProps {
    taskId: string;
}

export const CommentsTab: React.FC<CommentsTabProps> = ({ taskId }) => <CommentSection taskId={taskId} />;
