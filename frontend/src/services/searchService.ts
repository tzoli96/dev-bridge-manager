// frontend/src/services/searchService.ts
import { apiClient } from '@/lib/api'

export interface SearchClientResult {
    id: number
    name: string
}

export interface SearchProjectResult {
    id: number
    name: string
}

export interface SearchTaskResult {
    id: number
    title: string
    project_id: number
    project_name: string
}

export interface SearchResponse {
    success: boolean
    message?: string
    clients: SearchClientResult[]
    projects: SearchProjectResult[]
    tasks: SearchTaskResult[]
}

export const SearchService = {
    async search(q: string): Promise<SearchResponse> {
        return apiClient.get<SearchResponse>('/search', { q })
    },
}
