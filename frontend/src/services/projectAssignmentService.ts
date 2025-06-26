import { apiClient } from '@/lib/api'

export interface ProjectAssignment {
    id: number
    project_id: number
    user_id: number
    user_name: string
    user_email: string
    role: 'owner' | 'manager' | 'member' | 'viewer'
    assigned_at: string
    assigned_by: number
    assigned_by_name: string
    is_active: boolean
}

export interface ProjectAssignmentCreateRequest {
    user_id: number
    role?: 'owner' | 'manager' | 'member' | 'viewer'
}

export interface ProjectAssignmentUpdateRequest {
    role: 'owner' | 'manager' | 'member' | 'viewer'
}

export interface ProjectAssignmentResponse {
    success: boolean
    message: string
    assignments?: ProjectAssignment[]
    assignment?: ProjectAssignment
    count?: number
}

export class ProjectAssignmentService {
    private static getBaseUrl(projectId: number) {
        return `/projects/${projectId}/assignments`
    }

    static async getProjectAssignments(projectId: number): Promise<ProjectAssignment[]> {
        try {
            console.log('🔍 [ProjectAssignmentService] Fetching assignments for project:', projectId)

            const response = await apiClient.get<ProjectAssignmentResponse>(this.getBaseUrl(projectId))

            console.log('🔍 [ProjectAssignmentService] Raw response:', response)

            // Backend visszaadja a success: true-t és message-t, de üres assignments-szel
            if (response.success) {
                // Ha nincs assignments array, akkor üres tömböt adunk vissza
                const assignments = response.assignments || []
                console.log('🔍 [ProjectAssignmentService] Parsed assignments:', assignments)
                return assignments
            }

            // Csak akkor dobunk hibát, ha success: false
            throw new Error(response.message || 'Failed to fetch project assignments')
        } catch (error: any) {
            console.error('❌ [ProjectAssignmentService] Error fetching project assignments:', error)

            // Ha a response success: true volt, de hibát dobott, akkor network error
            if (error.message && error.message.includes('Project assignments retrieved successfully')) {
                console.log('🔍 [ProjectAssignmentService] Success message detected, returning empty array')
                return [] // Üres tömb a kezdeti állapothoz
            }

            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch project assignments')
        }
    }

    static async assignUserToProject(projectId: number, data: ProjectAssignmentCreateRequest): Promise<ProjectAssignment> {
        try {
            const response = await apiClient.post<ProjectAssignmentResponse>(this.getBaseUrl(projectId), data)

            if (response.success && response.assignment) {
                return response.assignment
            }

            throw new Error(response.message || 'Failed to assign user to project')
        } catch (error: any) {
            console.error('Error assigning user to project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to assign user to project')
        }
    }

    static async updateProjectAssignment(projectId: number, userId: number, data: ProjectAssignmentUpdateRequest): Promise<ProjectAssignment> {
        try {
            const response = await apiClient.put<ProjectAssignmentResponse>(`${this.getBaseUrl(projectId)}/${userId}`, data)

            if (response.success && response.assignment) {
                return response.assignment
            }

            throw new Error(response.message || 'Failed to update assignment')
        } catch (error: any) {
            console.error('Error updating assignment:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to update assignment')
        }
    }

    static async removeUserFromProject(projectId: number, userId: number): Promise<void> {
        try {
            const response = await apiClient.delete<ProjectAssignmentResponse>(`${this.getBaseUrl(projectId)}/${userId}`)

            if (!response.success) {
                throw new Error(response.message || 'Failed to remove user from project')
            }
        } catch (error: any) {
            console.error('Error removing user from project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to remove user from project')
        }
    }
}