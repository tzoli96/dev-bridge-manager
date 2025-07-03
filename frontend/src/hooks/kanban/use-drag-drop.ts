'use client';

import { useState, useCallback } from 'react';
import type { Task, DragState, DragDropResult } from '@/types/kanban';

interface UseDragDropReturn {
    dragState: DragState;
    isDragging: boolean;
    isColumnHighlighted: (columnId: string) => boolean;
    startDrag: (task: Task, fromColumnId: string) => void;
    endDrag: () => void;
    handleDragOver: (e: React.DragEvent) => void;
    handleDragEnter: (e: React.DragEvent, columnId: string) => void;
    handleDragLeave: (e: React.DragEvent) => void;
    handleDrop: (e: React.DragEvent, toColumnId: string) => DragDropResult | null;
}

export const useDragDrop = (): UseDragDropReturn => {
    const [dragState, setDragState] = useState<DragState>({
        isDragging: false,
        draggedTask: null,
        draggedFrom: null,
        dragOverColumn: null,
    });

    const startDrag = useCallback((task: Task, fromColumnId: string): void => {
        console.log('🎯 Drag started:', task.title, 'from:', fromColumnId);

        setDragState({
            isDragging: true,
            draggedTask: task,
            draggedFrom: fromColumnId,
            dragOverColumn: null,
        });
    }, []);

    const endDrag = useCallback((): void => {
        console.log('🏁 Drag ended');

        setDragState({
            isDragging: false,
            draggedTask: null,
            draggedFrom: null,
            dragOverColumn: null,
        });
    }, []);

    const handleDragOver = useCallback((e: React.DragEvent): void => {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
    }, []);

    const handleDragEnter = useCallback((e: React.DragEvent, columnId: string): void => {
        e.preventDefault();

        if (dragState.isDragging && dragState.draggedFrom !== columnId) {
            console.log('👆 Drag entered column:', columnId);

            setDragState(prev => ({
                ...prev,
                dragOverColumn: columnId,
            }));
        }
    }, [dragState.isDragging, dragState.draggedFrom]);

    const handleDragLeave = useCallback((e: React.DragEvent): void => {
        const target = e.currentTarget as HTMLElement;
        const relatedTarget = e.relatedTarget as HTMLElement;

        // Csak akkor távolítjuk el a highlight-ot, ha tényleg elhagyjuk az oszlopot
        if (!target.contains(relatedTarget)) {
            setDragState(prev => ({
                ...prev,
                dragOverColumn: null,
            }));
        }
    }, []);

    const handleDrop = useCallback((
        e: React.DragEvent,
        toColumnId: string
    ): DragDropResult | null => {
        e.preventDefault();
        e.stopPropagation();

        console.log('📦 Drop event fired for column:', toColumnId);

        // Reset drag over state
        setDragState(prev => ({
            ...prev,
            dragOverColumn: null,
        }));

        if (!dragState.draggedTask || !dragState.draggedFrom) {
            console.log('❌ No dragged task or source column');
            return null;
        }

        if (dragState.draggedFrom === toColumnId) {
            console.log('⚠️ Dropped on same column, no action needed');
            return null;
        }

        const result: DragDropResult = {
            taskId: dragState.draggedTask.id,
            fromColumnId: dragState.draggedFrom,
            toColumnId,
            // Position can be calculated based on drop position if needed
            newPosition: 0
        };

        console.log('✅ Drop result:', result);
        return result;
    }, [dragState]);

    const isColumnHighlighted = useCallback((columnId: string): boolean => {
        return dragState.dragOverColumn === columnId &&
            dragState.isDragging &&
            dragState.draggedFrom !== columnId;
    }, [dragState]);

    return {
        dragState,
        isDragging: dragState.isDragging,
        isColumnHighlighted,
        startDrag,
        endDrag,
        handleDragOver,
        handleDragEnter,
        handleDragLeave,
        handleDrop,
    };
};