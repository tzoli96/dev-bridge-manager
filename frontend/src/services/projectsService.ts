import { apiClient } from '@/lib/api'

export interface Project {
    id: number
    name: string
    description: string
    status: 'active' | 'completed' | 'on-hold' | 'cancelled'
    created_by: number
    created_by_name: string
    created_at: string
    updated_at: string
}

export interface ProjectCreateRequest {
    name: string
    description?: string
    status?: 'active' | 'completed' | 'on-hold' | 'cancelled'
}

export interface ProjectUpdateRequest {
    name?: string
    description?: string
    status?: 'active' | 'completed' | 'on-hold' | 'cancelled'
}

export interface ProjectsResponse {
    success: boolean
    message: string
    projects?: Project[]
    project?: Project
    count?: number
}

export class ProjectsService {
    private static baseUrl = '/projects'  // Eltávolítottam a /api/v1 részt

    static async getAllProjects(): Promise<Project[]> {
        try {
            const response = await apiClient.get<ProjectsResponse>(this.baseUrl)

            if (response.data.success && response.data.projects) {
                return response.data.projects
            }

            throw new Error(response.data.message || 'Failed to fetch projects')
        } catch (error: any) {
            console.error('Error fetching projects:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch projects')
        }
    }

    static async getProject(id: number): Promise<Project> {
        try {
            const response = await apiClient.get<ProjectsResponse>(`${this.baseUrl}/${id}`)

            if (response.data.success && response.data.project) {
                return response.data.project
            }

            throw new Error(response.data.message || 'Failed to fetch project')
        } catch (error: any) {
            console.error('Error fetching project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch project')
        }
    }

    static async createProject(projectData: ProjectCreateRequest): Promise<Project> {
        try {
            const response = await apiClient.post<ProjectsResponse>(this.baseUrl, projectData)

            if (response.data.success && response.data.project) {
                return response.data.project
            }

            throw new Error(response.data.message || 'Failed to create project')
        } catch (error: any) {
            console.error('Error creating project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to create project')
        }
    }

    static async updateProject(id: number, projectData: ProjectUpdateRequest): Promise<Project> {
        try {
            const response = await apiClient.put<ProjectsResponse>(`${this.baseUrl}/${id}`, projectData)

            if (response.data.success && response.data.project) {
                return response.data.project
            }

            throw new Error(response.data.message || 'Failed to update project')
        } catch (error: any) {
            console.error('Error updating project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to update project')
        }
    }

    static async deleteProject(id: number): Promise<void> {
        try {
            const response = await apiClient.delete<ProjectsResponse>(`${this.baseUrl}/${id}`)

            if (!response.data.success) {
                throw new Error(response.data.message || 'Failed to delete project')
            }
        } catch (error: any) {
            console.error('Error deleting project:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to delete project')
        }
    }
}