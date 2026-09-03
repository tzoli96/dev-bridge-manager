import type { Task } from './task.types';

export interface KanbanColumn {
    id: string;
    title: string;
    color: string;
    position: number;
    maxTasks?: number;
    tasks: Task[];
    createdAt: string;
    updatedAt: string;
}

export interface KanbanBoard {
    id: string;
    projectId: string;
    name: string;
    columns: KanbanColumn[];
    settings: KanbanSettings;
    createdAt: string;
    updatedAt: string;
}

export interface Board {
    id: string;
    projectId: string;
    name: string;
    position: number;
    createdAt: string;
    updatedAt: string;
}

export interface CreateBoardData {
    name: string;
    position?: number;
}

export interface UpdateBoardData {
    name?: string;
    position?: number;
}

export interface KanbanSettings {
    enableWipLimits: boolean;
    enableTimeTracking: boolean;
    enableComments: boolean;
    enablePriorities: boolean;
    enableTags: boolean;
    defaultEstimateUnit: 'hours' | 'days' | 'points';
}

export interface KanbanPermissions {
    canCreateTasks: boolean;
    canEditTasks: boolean;
    canDeleteTasks: boolean;
    canMoveTasks: boolean;
    canManageColumns: boolean;
    canViewTimeTracking: boolean;
    canEditTimeTracking: boolean;
}

export enum ModalType {
    TASK_EDIT = 'task_edit',
    COMMENTS = 'comments',
    TIME_LOG = 'time_log',
    COLUMN_SETTINGS = 'column_settings',
    ADD_TO_BOARD = 'add_to_board',
}

export interface DragState {
    isDragging: boolean;
    draggedTask: Task | null;
    draggedFrom: string | null;
    dragOverColumn: string | null;
}

export interface DragDropResult {
    taskId: string;
    fromColumnId: string;
    toColumnId: string;
    newPosition?: number;
}