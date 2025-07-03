export enum TaskPriority {
    LOW = 'low',
    MEDIUM = 'medium',
    HIGH = 'high',
    URGENT = 'urgent'
}

export enum TaskStatus {
    TODO = 'todo',
    IN_PROGRESS = 'in_progress',
    REVIEW = 'review',
    DONE = 'done'
}

export interface Task {
    id: string;
    title: string;
    description: string;
    htmlDescription?: string;
    priority: TaskPriority;
    status: TaskStatus;
    columnId: string;
    projectId: string;
    assigneeId?: string;
    assignee?: TaskAssignee;
    estimatedHours: number;
    loggedHours: number;
    tags: TaskTag[];
    timeEntries: TimeEntry[];
    comments: TaskComment[];
    attachments: TaskAttachment[];
    position: number;
    dueDate?: string;
    createdAt: string;
    updatedAt: string;
    createdBy: string;
    updatedBy: string;
}

export interface TaskAssignee {
    id: string;
    name: string;
    email: string;
    avatar?: string;
}

export interface TaskTag {
    id: string;
    name: string;
    color: string;
}

export interface TimeEntry {
    id: string;
    taskId: string;
    hours: number;
    description: string;
    date: string;
    userId: string;
    user?: {
        id: string;
        name: string;
        avatar?: string;
    };
    createdAt: string;
    updatedAt: string;
}

export interface TaskComment {
    id: string;
    taskId: string;
    content: string;
    htmlContent?: string;
    userId: string;
    user?: {
        id: string;
        name: string;
        avatar?: string;
    };
    isEdited: boolean;
    createdAt: string;
    updatedAt: string;
}

export interface TaskAttachment {
    id: string;
    taskId: string;
    filename: string;
    originalName: string;
    mimeType: string;
    size: number;
    url: string;
    uploadedBy: string;
    createdAt: string;
}

// API Request/Response types
export interface CreateTaskData {
    title: string;
    description: string;
    htmlDescription?: string;
    priority: TaskPriority;
    columnId: string;
    assigneeId?: string;
    estimatedHours?: number;
    tags?: string[];
    dueDate?: string;
}

export interface UpdateTaskData {
    title?: string;
    description?: string;
    htmlDescription?: string;
    priority?: TaskPriority;
    assigneeId?: string;
    estimatedHours?: number;
    tags?: string[];
    dueDate?: string;
}

export interface MoveTaskData {
    columnId: string;
    position: number;
}

export interface CreateTimeEntryData {
    hours: number;
    description: string;
    date: string;
}

export interface UpdateTimeEntryData {
    hours?: number;
    description?: string;
    date?: string;
}

export interface CreateCommentData {
    content: string;
    htmlContent?: string;
}

export interface UpdateCommentData {
    content: string;
    htmlContent?: string;
}

// Form types
export interface TaskFormData {
    title: string;
    description: string;
    htmlDescription: string;
    priority: TaskPriority;
    assigneeId: string;
    estimatedHours: number;
    tags: string[];
    dueDate: string;
}

export interface TimeEntryFormData {
    hours: number;
    description: string;
    date: string;
}

export interface CommentFormData {
    content: string;
    htmlContent: string;
}

// Filter and sort types
export interface TaskFilters {
    assigneeIds?: string[];
    priorities?: TaskPriority[];
    tags?: string[];
    hasEstimate?: boolean;
    isOverdue?: boolean;
    search?: string;
}

export interface TaskSortOptions {
    field: 'createdAt' | 'updatedAt' | 'priority' | 'dueDate' | 'estimatedHours' | 'title';
    direction: 'asc' | 'desc';
}