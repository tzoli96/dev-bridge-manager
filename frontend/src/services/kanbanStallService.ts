// frontend/src/services/kanbanStallService.ts
import { apiClient } from '@/lib/api'

export interface KanbanStallFlag {
    id: number
    task_id: number
    project_id: number
    board_id: number
    column_id: number
    days_stalled: number
    status: 'pending' | 'dismissed'
    created_at: string
    dismissed_by: number | null
    dismissed_at: string | null
}

export interface KanbanStallFlagWithNames extends KanbanStallFlag {
    task_title: string
    project_name: string
    board_name: string
    column_title: string
}

interface ListApiResponse {
    success: boolean
    message?: string
    flags?: KanbanStallFlagWithNames[]
}

interface ActionApiResponse {
    success: boolean
    message?: string
    flag?: KanbanStallFlag
}

export const KanbanStallService = {
    async list(status?: 'pending' | 'dismissed'): Promise<KanbanStallFlagWithNames[]> {
        const response = await apiClient.get<ListApiResponse>(
            '/admin/kanban-stall-flags',
            status ? { status } : undefined
        )
        if (response.success) return response.flags || []
        throw new Error(response.message || 'Failed to fetch stalled task flags')
    },

    async dismiss(id: number): Promise<void> {
        const response = await apiClient.post<ActionApiResponse>(`/admin/kanban-stall-flags/${id}/dismiss`, {})
        if (!response.success) throw new Error(response.message || 'Failed to dismiss flag')
    },
}
