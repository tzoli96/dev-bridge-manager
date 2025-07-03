import type { Task } from './task.types';

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

export interface DragHandlers {
    onDragStart: (e: React.DragEvent, task: Task, fromColumnId: string) => void;
    onDragEnd: (e: React.DragEvent) => void;
    onDragOver: (e: React.DragEvent) => void;
    onDragEnter: (e: React.DragEvent, columnId: string) => void;
    onDragLeave: (e: React.DragEvent) => void;
    onDrop: (e: React.DragEvent, toColumnId: string) => void;
}

// Frissítsd a types/kanban/index.ts fájlt is:
// export * from './drag-drop.types';