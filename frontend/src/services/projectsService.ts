import { apiClient } from '@/lib/api'

export interface ProjectClient {
    id: number
    project_id: number
    client_id: number
    client_name: string
    client_type: string
    assigned_at: string
    assigned_by: number
    assigned_by_name: string
}

export interface Project {
    id: number
    name: string
    description: string
    status: 'active' | 'completed' | 'on-hold' | 'cancelled'
    pricing_type?: 'hourly' | 'fixed' | 'hobby' | ''
    hourly_rate?: number | null
    fixed_price?: number | null
    auto_invoice_enabled?: boolean
    auto_invoice_client_id?: number | null
    auto_invoice_auto_approve?: boolean
    clients?: ProjectClient[]
    created_by: number
    created_by_name: string
    created_at: string
    updated_at: string
}

export interface ProjectCreateRequest {
    name: string
    description?: string
    status?: 'active' | 'completed' | 'on-hold' | 'cancelled'
    pricing_type?: 'hourly' | 'fixed' | 'hobby' | ''
    hourly_rate?: number | null
    fixed_price?: number | null
}

export interface ProjectUpdateRequest {
    name?: string
    description?: string
    status?: 'active' | 'completed' | 'on-hold' | 'cancelled'
    pricing_type?: 'hourly' | 'fixed' | 'hobby' | ''
    hourly_rate?: number | null
    fixed_price?: number | null
    auto_invoice_enabled?: boolean
    auto_invoice_client_id?: number | null
    auto_invoice_auto_approve?: boolean
}

export interface ProjectsResponse {
    success: boolean
    message: string
    projects?: Project[]
    project?: Project
    count?: number
}

export interface ProjectClientsResponse {
    success: boolean
    message: string
    project_clients?: ProjectClient[]
    project_client?: ProjectClient
    count?: number
}

export class ProjectsService {
    private static baseUrl = '/projects'

    static async getAllProjects(): Promise<Project[]> {
        try {
            const response = await apiClient.get<ProjectsResponse>(this.baseUrl)

            if (response.success) {
                return response.projects || []
            }

            throw new Error(response.message || 'Failed to fetch projects')
        } catch (error: any) {
            console.error('Error fetching projects:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch projects')
        }
    }

    static async getProject(id: number): Promise<Project> {
        try {
            const response = await apiClient.get<ProjectsResponse>(`${this.baseUrl}/${id}`)

            if (response.success && response.project) {
                return response.project
            }

            throw new Error(response.message || 'Failed to fetch project')
        } catch (error: any) {
            console.error('Error fetching project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch project')
        }
    }

    static async createProject(projectData: ProjectCreateRequest): Promise<Project> {
        try {
            const response = await apiClient.post<ProjectsResponse>(this.baseUrl, projectData)

            if (response.success && response.project) {
                return response.project
            }

            throw new Error(response.message || 'Failed to create project')
        } catch (error: any) {
            console.error('Error creating project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to create project')
        }
    }

    static async updateProject(id: number, projectData: ProjectUpdateRequest): Promise<Project> {
        try {
            const response = await apiClient.put<ProjectsResponse>(`${this.baseUrl}/${id}`, projectData)

            if (response.success && response.project) {
                return response.project
            }

            throw new Error(response.message || 'Failed to update project')
        } catch (error: any) {
            console.error('Error updating project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to update project')
        }
    }

    static async deleteProject(id: number): Promise<void> {
        try {
            const response = await apiClient.delete<ProjectsResponse>(`${this.baseUrl}/${id}`)

            if (!response.success) {
                throw new Error(response.message || 'Failed to delete project')
            }
        } catch (error: any) {
            console.error('Error deleting project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to delete project')
        }
    }

    static async getProjectClients(projectId: number): Promise<ProjectClient[]> {
        try {
            const response = await apiClient.get<ProjectClientsResponse>(`${this.baseUrl}/${projectId}/clients`)

            if (response.success) {
                return response.project_clients || []
            }

            throw new Error(response.message || 'Failed to fetch project clients')
        } catch (error: any) {
            console.error('Error fetching project clients:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch project clients')
        }
    }

    static async assignClientToProject(projectId: number, clientId: number): Promise<ProjectClient> {
        try {
            const response = await apiClient.post<ProjectClientsResponse>(`${this.baseUrl}/${projectId}/clients`, { client_id: clientId })

            if (response.success && response.project_client) {
                return response.project_client
            }

            throw new Error(response.message || 'Failed to assign client to project')
        } catch (error: any) {
            console.error('Error assigning client to project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to assign client to project')
        }
    }

    static async removeClientFromProject(projectId: number, clientId: number): Promise<void> {
        try {
            const response = await apiClient.delete<ProjectClientsResponse>(`${this.baseUrl}/${projectId}/clients/${clientId}`)

            if (!response.success) {
                throw new Error(response.message || 'Failed to remove client from project')
            }
        } catch (error: any) {
            console.error('Error removing client from project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to remove client from project')
        }
    }
}