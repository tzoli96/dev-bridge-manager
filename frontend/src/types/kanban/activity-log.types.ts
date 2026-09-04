export interface ActivityLogEntry {
    id: string;
    taskId: string;
    userId: string;
    user?: {
        id: string;
        name: string;
        avatar?: string;
    };
    eventType: 'field_changed' | 'moved' | 'comment_added' | 'comment_deleted' | 'attachment_added' | 'attachment_deleted';
    fieldName?: string;
    oldValue?: string;
    newValue?: string;
    createdAt: string;
}
