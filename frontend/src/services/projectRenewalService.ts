// frontend/src/services/projectRenewalService.ts
import { apiClient } from '@/lib/api'

export interface ProjectRenewalFlag {
    id: number
    project_id: number
    contract_end_date: string
    status: 'pending' | 'dismissed'
    created_at: string
    dismissed_by: number | null
    dismissed_at: string | null
}

export interface ProjectRenewalFlagWithNames extends ProjectRenewalFlag {
    project_name: string
}

interface ListApiResponse {
    success: boolean
    message?: string
    flags?: ProjectRenewalFlagWithNames[]
}

interface ActionApiResponse {
    success: boolean
    message?: string
    flag?: ProjectRenewalFlag
}

export const ProjectRenewalService = {
    async list(status?: 'pending' | 'dismissed'): Promise<ProjectRenewalFlagWithNames[]> {
        const response = await apiClient.get<ListApiResponse>(
            '/admin/project-renewal-flags',
            status ? { status } : undefined
        )
        if (response.success) return response.flags || []
        throw new Error(response.message || 'Failed to fetch project renewal flags')
    },

    async dismiss(id: number): Promise<void> {
        const response = await apiClient.post<ActionApiResponse>(`/admin/project-renewal-flags/${id}/dismiss`, {})
        if (!response.success) throw new Error(response.message || 'Failed to dismiss flag')
    },
}
