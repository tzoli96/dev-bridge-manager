import { useState, useEffect, useCallback } from 'react'
import { ProjectsService, Project } from '@/services/projectsService'

export const useProjects = () => {
    const [projects, setProjects] = useState<Project[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const fetchProjects = useCallback(async () => {
        try {
            setLoading(true)
            setError(null)
            const projectsData = await ProjectsService.getAllProjects()
            setProjects(projectsData)
        } catch (err: any) {
            setError(err.message)
            setProjects([])
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        fetchProjects()
    }, [fetchProjects])

    const refetch = useCallback(() => {
        return fetchProjects()
    }, [fetchProjects])

    return {
        projects,
        loading,
        error,
        refetch
    }
}