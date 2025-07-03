import { create } from 'zustand';
import { devtools, subscribeWithSelector } from 'zustand/middleware';
import { immer } from 'zustand/middleware/immer';
import type {
    Task,
    TaskComment,
    TimeEntry,
    KanbanBoard,
    KanbanColumn
} from '@/types/kanban';

interface KanbanState {
    // Board state
    board: KanbanBoard | null;
    columns: KanbanColumn[];

    // Tasks state
    tasks: Task[];
    selectedTaskId: string | null;

    // Comments state
    comments: TaskComment[];

    // Time entries state
    timeEntries: TimeEntry[];

    // UI state
    isLoading: boolean;
    error: string | null;

    // Optimistic updates
    optimisticUpdates: Map<string, any>;
}

interface KanbanActions {
    // Board actions
    setBoard: (board: KanbanBoard) => void;
    updateBoard: (updates: Partial<KanbanBoard>) => void;
    clearBoard: () => void;

    // Column actions
    setColumns: (columns: KanbanColumn[]) => void;
    addColumn: (column: KanbanColumn) => void;
    updateColumn: (columnId: string, updates: Partial<KanbanColumn>) => void;
    deleteColumn: (columnId: string) => void;
    reorderColumns: (columnOrders: { columnId: string; position: number }[]) => void;

    // Task actions
    setTasks: (tasks: Task[]) => void;
    addTask: (task: Task) => void;
    updateTaskInStore: (task: Task) => void;
    deleteTaskFromStore: (taskId: string) => void;
    moveTaskInStore: (taskId: string, toColumnId: string, position?: number) => void;
    selectTask: (taskId: string | null) => void;

    // Comment actions
    setComments: (comments: TaskComment[]) => void;
    addCommentToStore: (comment: TaskComment) => void;
    updateCommentInStore: (comment: TaskComment) => void;
    deleteCommentFromStore: (commentId: string) => void;

    // Time entry actions
    setTimeEntries: (entries: TimeEntry[]) => void;
    addTimeEntryToStore: (entry: TimeEntry) => void;
    updateTimeEntryInStore: (entry: TimeEntry) => void;
    deleteTimeEntryFromStore: (entryId: string) => void;

    // UI actions
    setLoading: (loading: boolean) => void;
    setError: (error: string | null) => void;

    // Optimistic updates
    addOptimisticUpdate: (id: string, data: any) => void;
    removeOptimisticUpdate: (id: string) => void;
    clearOptimisticUpdates: () => void;

    // Utility actions
    reset: () => void;
    hydrate: (data: Partial<KanbanState>) => void;
}

type KanbanStore = KanbanState & KanbanActions;

const initialState: KanbanState = {
    board: null,
    columns: [],
    tasks: [],
    selectedTaskId: null,
    comments: [],
    timeEntries: [],
    isLoading: false,
    error: null,
    optimisticUpdates: new Map(),
};

export const useKanbanStore = create<KanbanStore>()(
    devtools(
        subscribeWithSelector(
            immer((set, get) => ({
                ...initialState,

                // Board actions
                setBoard: (board) => set((state) => {
                    state.board = board;
                    state.columns = board.columns;
                }),

                updateBoard: (updates) => set((state) => {
                    if (state.board) {
                        Object.assign(state.board, updates);
                    }
                }),

                clearBoard: () => set((state) => {
                    state.board = null;
                    state.columns = [];
                }),

                // Column actions
                setColumns: (columns) => set((state) => {
                    state.columns = columns;
                    if (state.board) {
                        state.board.columns = columns;
                    }
                }),

                addColumn: (column) => set((state) => {
                    state.columns.push(column);
                    if (state.board) {
                        state.board.columns.push(column);
                    }
                }),

                updateColumn: (columnId, updates) => set((state) => {
                    const columnIndex = state.columns.findIndex(col => col.id === columnId);
                    if (columnIndex !== -1) {
                        Object.assign(state.columns[columnIndex], updates);
                    }
                    if (state.board) {
                        const boardColumnIndex = state.board.columns.findIndex(col => col.id === columnId);
                        if (boardColumnIndex !== -1) {
                            Object.assign(state.board.columns[boardColumnIndex], updates);
                        }
                    }
                }),

                deleteColumn: (columnId) => set((state) => {
                    state.columns = state.columns.filter(col => col.id !== columnId);
                    state.tasks = state.tasks.filter(task => task.columnId !== columnId);
                    if (state.board) {
                        state.board.columns = state.board.columns.filter(col => col.id !== columnId);
                    }
                }),

                reorderColumns: (columnOrders) => set((state) => {
                    const orderedColumns = columnOrders
                        .sort((a, b) => a.position - b.position)
                        .map(order => state.columns.find(col => col.id === order.columnId))
                        .filter(Boolean) as KanbanColumn[];

                    state.columns = orderedColumns;
                    if (state.board) {
                        state.board.columns = orderedColumns;
                    }
                }),

                // Task actions
                setTasks: (tasks) => set((state) => {
                    state.tasks = tasks;
                }),

                addTask: (task) => set((state) => {
                    state.tasks.push(task);

                    // Add to appropriate column
                    const column = state.columns.find(col => col.id === task.columnId);
                    if (column) {
                        column.tasks.push(task);
                    }
                }),

                updateTaskInStore: (updatedTask) => set((state) => {
                    const taskIndex = state.tasks.findIndex(task => task.id === updatedTask.id);
                    if (taskIndex !== -1) {
                        state.tasks[taskIndex] = updatedTask;
                    }

                    // Update in columns
                    state.columns.forEach(column => {
                        const columnTaskIndex = column.tasks.findIndex(task => task.id === updatedTask.id);
                        if (columnTaskIndex !== -1) {
                            column.tasks[columnTaskIndex] = updatedTask;
                        }
                    });
                }),

                deleteTaskFromStore: (taskId) => set((state) => {
                    state.tasks = state.tasks.filter(task => task.id !== taskId);
                    state.comments = state.comments.filter(comment => comment.taskId !== taskId);
                    state.timeEntries = state.timeEntries.filter(entry => entry.taskId !== taskId);

                    // Remove from columns
                    state.columns.forEach(column => {
                        column.tasks = column.tasks.filter(task => task.id !== taskId);
                    });

                    if (state.selectedTaskId === taskId) {
                        state.selectedTaskId = null;
                    }
                }),

                moveTaskInStore: (taskId, toColumnId, position) => set((state) => {
                    const task = state.tasks.find(t => t.id === taskId);
                    if (!task) return;

                    const fromColumnId = task.columnId;

                    // Update task's columnId
                    task.columnId = toColumnId;
                    task.position = position || 0;

                    // Remove from old column
                    const fromColumn = state.columns.find(col => col.id === fromColumnId);
                    if (fromColumn) {
                        fromColumn.tasks = fromColumn.tasks.filter(t => t.id !== taskId);
                    }

                    // Add to new column
                    const toColumn = state.columns.find(col => col.id === toColumnId);
                    if (toColumn) {
                        if (position !== undefined) {
                            toColumn.tasks.splice(position, 0, task);
                        } else {
                            toColumn.tasks.push(task);
                        }
                    }
                }),

                selectTask: (taskId) => set((state) => {
                    state.selectedTaskId = taskId;
                }),

                // Comment actions
                setComments: (comments) => set((state) => {
                    state.comments = comments;
                }),

                addCommentToStore: (comment) => set((state) => {
                    state.comments.push(comment);
                }),

                updateCommentInStore: (updatedComment) => set((state) => {
                    const commentIndex = state.comments.findIndex(comment => comment.id === updatedComment.id);
                    if (commentIndex !== -1) {
                        state.comments[commentIndex] = updatedComment;
                    }
                }),

                deleteCommentFromStore: (commentId) => set((state) => {
                    state.comments = state.comments.filter(comment => comment.id !== commentId);
                }),

                // Time entry actions
                setTimeEntries: (entries) => set((state) => {
                    state.timeEntries = entries;
                }),

                addTimeEntryToStore: (entry) => set((state) => {
                    state.timeEntries.push(entry);
                }),

                updateTimeEntryInStore: (updatedEntry) => set((state) => {
                    const entryIndex = state.timeEntries.findIndex(entry => entry.id === updatedEntry.id);
                    if (entryIndex !== -1) {
                        state.timeEntries[entryIndex] = updatedEntry;
                    }
                }),

                deleteTimeEntryFromStore: (entryId) => set((state) => {
                    state.timeEntries = state.timeEntries.filter(entry => entry.id !== entryId);
                }),

                // UI actions
                setLoading: (loading) => set((state) => {
                    state.isLoading = loading;
                }),

                setError: (error) => set((state) => {
                    state.error = error;
                }),

                // Optimistic updates
                addOptimisticUpdate: (id, data) => set((state) => {
                    state.optimisticUpdates.set(id, data);
                }),

                removeOptimisticUpdate: (id) => set((state) => {
                    state.optimisticUpdates.delete(id);
                }),

                clearOptimisticUpdates: () => set((state) => {
                    state.optimisticUpdates.clear();
                }),

                // Utility actions
                reset: () => set(() => ({ ...initialState })),

                hydrate: (data) => set((state) => {
                    Object.assign(state, data);
                }),
            }))
        ),
        {
            name: 'kanban-store',
        }
    )
);

// Selectors for better performance
export const useKanbanSelectors = () => {
    const store = useKanbanStore();

    return {
        // Board selectors
        board: store.board,
        columns: store.columns,
        isLoading: store.isLoading,
        error: store.error,

        // Task selectors
        tasks: store.tasks,
        selectedTask: store.selectedTaskId ? store.tasks.find(t => t.id === store.selectedTaskId) : null,
        getTasksByColumn: (columnId: string) =>
            store.tasks.filter(task => task.columnId === columnId)
                .sort((a, b) => a.position - b.position),

        // Comment selectors
        getCommentsByTask: (taskId: string) =>
            store.comments.filter(comment => comment.taskId === taskId)
                .sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()),

        // Time entry selectors
        getTimeEntriesByTask: (taskId: string) =>
            store.timeEntries.filter(entry => entry.taskId === taskId)
                .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()),

        getTotalHoursByTask: (taskId: string) =>
            store.timeEntries
                .filter(entry => entry.taskId === taskId)
                .reduce((total, entry) => total + entry.hours, 0),

        // Statistics selectors
        getTaskCountByColumn: (columnId: string) =>
            store.tasks.filter(task => task.columnId === columnId).length,

        getTotalTasks: () => store.tasks.length,

        getTasksByPriority: (priority: string) =>
            store.tasks.filter(task => task.priority === priority),

        getOverdueTasks: () => {
            const now = new Date();
            return store.tasks.filter(task =>
                task.dueDate && new Date(task.dueDate) < now
            );
        },

        getTasksWithEstimates: () =>
            store.tasks.filter(task => task.estimatedHours > 0),

        getTotalEstimatedHours: () =>
            store.tasks.reduce((total, task) => total + task.estimatedHours, 0),

        getTotalLoggedHours: () =>
            store.timeEntries.reduce((total, entry) => total + entry.hours, 0),
    };
};

// Persistence middleware for selected data
if (typeof window !== 'undefined') {
    useKanbanStore.subscribe(
        (state) => ({
            selectedTaskId: state.selectedTaskId,
            // Add other UI state that should persist
        }),
        (persistedState) => {
            localStorage.setItem('kanban-ui-state', JSON.stringify(persistedState));
        }
    );

    // Restore persisted state on load
    const persistedState = localStorage.getItem('kanban-ui-state');
    if (persistedState) {
        try {
            const parsed = JSON.parse(persistedState);
            useKanbanStore.getState().hydrate(parsed);
        } catch (error) {
            console.warn('Failed to restore kanban UI state:', error);
        }
    }
}