'use client'

import { User } from '@/types/user'
import { useProjects } from '@/hooks/useProjects'
import { useModals } from '@/components/dashboard/DashboardModals'
import { ProjectsService, Project } from '@/services/projectsService'
import { isAdmin } from '@/utils/permissions'
import { useState } from 'react'
import ProjectsGrid from '@/components/dashboard/ProjectsGrid'
import EmptyState from '@/components/ui/EmptyState'
import ErrorState from '@/components/ui/ErrorState'
import LoadingState from '@/components/ui/LoadingState'

interface ProjectsTabProps {
    user: User | null
}

export default function ProjectsTab({ user }: ProjectsTabProps) {
    const { projects, loading, error, refetch } = useProjects()
    const { setShowCreateProject, setShowEditProject, setSelectedProject } = useModals()
    const [deleting, setDeleting] = useState<number | null>(null)

    const handleDeleteProject = async (projectId: number) => {
        if (!confirm('Are you sure you want to delete this project?')) return

        try {
            setDeleting(projectId)
            await ProjectsService.deleteProject(projectId)
            await refetch()
        } catch (error: any) {
            alert(`Failed to delete project: ${error.message}`)
        } finally {
            setDeleting(null)
        }
    }

    const handleEditProject = (project: Project) => {
        setSelectedProject(project)
        setShowEditProject(true)
    }

    if (loading) return <LoadingState message="Loading projects..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h2 className="text-lg font-semibold">Projects</h2>
                    <p className="text-gray-600">View and manage project portfolio</p>
                </div>
                {isAdmin(user) && (
                    <button
                        onClick={() => setShowCreateProject(true)}
                        className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                    >
                        Create New Project
                    </button>
                )}
            </div>

            {!projects || projects.length === 0 ? (
                <EmptyState
                    icon="projects"
                    title="No projects found"
                    description="There are no projects to display."
                    action={isAdmin(user) ? {
                        label: "Create First Project",
                        onClick: () => setShowCreateProject(true)
                    } : undefined}
                />
            ) : (
                <ProjectsGrid
                    projects={projects}
                    user={user}
                    onEditProject={handleEditProject}
                    onDeleteProject={handleDeleteProject}
                    deleting={deleting}
                />
            )}
        </div>
    )
}