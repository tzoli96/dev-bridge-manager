import type { Task } from '@/types/kanban'

export interface DueTask {
    task: Task
    dueDateKey: string
    isOverdue: boolean
}

export function computeDueTasks(tasks: Task[], limit = 5): { overdue: DueTask[]; upcoming: DueTask[] } {
    const todayKey = new Date().toISOString().slice(0, 10)
    const dueTasks = tasks
        .filter((t) => t.dueDate && t.status !== 'done')
        .map((t) => ({
            task: t,
            dueDateKey: t.dueDate!.slice(0, 10),
            isOverdue: t.dueDate!.slice(0, 10) < todayKey,
        }))
        .sort((a, b) => a.dueDateKey.localeCompare(b.dueDateKey))

    return {
        overdue: dueTasks.filter((d) => d.isOverdue).slice(0, limit),
        upcoming: dueTasks.filter((d) => !d.isOverdue).slice(0, limit),
    }
}

export function computeTaskStats(tasks: Task[]) {
    const todayKey = new Date().toISOString().slice(0, 10)
    const in7Days = new Date()
    in7Days.setDate(in7Days.getDate() + 7)
    const in7DaysKey = in7Days.toISOString().slice(0, 10)

    const openTasks = tasks.filter((t) => t.status !== 'done')
    const doneTasks = tasks.filter((t) => t.status === 'done')
    const overdueTasks = openTasks.filter((t) => t.dueDate && t.dueDate.slice(0, 10) < todayKey)
    const dueSoonTasks = openTasks.filter(
        (t) => t.dueDate && t.dueDate.slice(0, 10) >= todayKey && t.dueDate.slice(0, 10) <= in7DaysKey
    )

    const byBoard = new Map<string, { total: number; open: number }>()
    for (const t of tasks) {
        if (!t.boardId) continue
        const entry = byBoard.get(t.boardId) || { total: 0, open: 0 }
        entry.total += 1
        if (t.status !== 'done') entry.open += 1
        byBoard.set(t.boardId, entry)
    }

    return {
        totalTasks: tasks.length,
        openTasksCount: openTasks.length,
        doneTasksCount: doneTasks.length,
        overdueCount: overdueTasks.length,
        dueSoonCount: dueSoonTasks.length,
        byBoard,
    }
}
