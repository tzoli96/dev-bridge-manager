export interface TaskCardProps {
    task: Task;
    isEditing: boolean;
    permissions: KanbanPermissions;
    onEdit: () => void;
    onSave: (data: UpdateTaskData) => void;
    onCancel: () => void;
    onDelete: () => void;
    onOpenComments: () => void;
    onOpenTimeLog: () => void;
}

export interface KanbanColumnProps {
    column: KanbanColumn;
    tasks: Task[];
    permissions: KanbanPermissions;
    isHighlighted: boolean;
    onAddTask: () => void;
    onEditTask: (taskId: string) => void;
    onDeleteTask: (taskId: string) => void;
    onMoveTask: (result: DragDropResult) => void;
}

export interface TaskFormProps {
    initialData?: Partial<TaskFormData>;
    onSubmit: (data: TaskFormData) => void;
    onCancel: () => void;
    isLoading?: boolean;
}

export interface CommentSectionProps {
    taskId: string;
    comments: TaskComment[];
    permissions: KanbanPermissions;
    onAddComment: (data: CreateCommentData) => void;
    onUpdateComment: (commentId: string, data: UpdateCommentData) => void;
    onDeleteComment: (commentId: string) => void;
}

export interface TimeTrackerProps {
    taskId: string;
    timeEntries: TimeEntry[];
    estimatedHours: number;
    loggedHours: number;
    permissions: KanbanPermissions;
    onAddTimeEntry: (data: CreateTimeEntryData) => void;
    onUpdateTimeEntry: (entryId: string, data: UpdateTimeEntryData) => void;
    onDeleteTimeEntry: (entryId: string) => void;
    onUpdateEstimate: (hours: number) => void;
}

export interface ProgressBarProps {
    percentage: number;
    size?: 'sm' | 'md' | 'lg';
    variant?: 'default' | 'success' | 'warning' | 'danger';
    showLabel?: boolean;
    className?: string;
}

export interface RichTextEditorProps {
    content: string;
    placeholder?: string;
    minHeight?: string;
    onChange: (html: string, text: string) => void;
    onFocus?: () => void;
    onBlur?: () => void;
    disabled?: boolean;
    className?: string;
}