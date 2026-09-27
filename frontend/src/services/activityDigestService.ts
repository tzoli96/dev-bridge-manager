// frontend/src/services/activityDigestService.ts
import { apiClient } from '@/lib/api'

export interface ActivityDigestTask {
    id: number
    title: string
    project_name: string
}

export interface ActivityDigest {
    success: boolean
    new_tasks_count: number
    new_tasks: ActivityDigestTask[]
    completed_tasks_count: number
    completed_tasks: ActivityDigestTask[]
    new_invoices_count: number
    new_invoices_total: number
    new_client_emails_count: number
    new_stall_flags_count: number
}

export const ActivityDigestService = {
    async get(): Promise<ActivityDigest> {
        return apiClient.get<ActivityDigest>('/activity-digest')
    },
}
