'use client'

import { useRouter } from 'next/navigation'
import { useEffect } from 'react'
import { useProjects } from '@/hooks/useProjects'
import { useModals } from '@/components/dashboard/DashboardModals'
import { useAuth } from '@/contexts/AuthContext'
import { isAdmin } from '@/utils/permissions'
import { Button } from '@/components/ui/button'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import EmptyState from '@/components/ui/EmptyState'

const statusStyles: Record<string, string> = {
    active: 'bg-success/10 text-success',
    completed: 'bg-primary/10 text-primary',
    'on-hold': 'bg-warning/10 text-warning',
    cancelled: 'bg-muted text-muted-foreground',
}

function formatPricing(project: { pricing_type?: 'hourly' | 'fixed' | ''; hourly_rate?: number | null; fixed_price?: number | null }): string {
    if (project.pricing_type === 'hourly' && project.hourly_rate) {
        return `${project.hourly_rate.toLocaleString()} HUF/hr`
    }
    if (project.pricing_type === 'fixed' && project.fixed_price) {
        return `${project.fixed_price.toLocaleString()} HUF fixed`
    }
    return '-'
}

export default function BoardPage() {
    const router = useRouter()
    const { user } = useAuth()
    const { projects, loading, error, refetch } = useProjects()
    const { setShowCreateProject, setOnProjectUpdated } = useModals()

    useEffect(() => {
        setOnProjectUpdated(() => refetch)
        return () => setOnProjectUpdated(undefined)
    }, [setOnProjectUpdated, refetch])

    if (loading) return <LoadingState message="Loading projects..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">Projects</h1>
                    <p className="text-muted-foreground text-sm">Choose a project to open its boards</p>
                </div>
                {isAdmin(user) && (
                    <Button onClick={() => setShowCreateProject(true)}>
                        New Project
                    </Button>
                )}
            </div>

            {!projects || projects.length === 0 ? (
                <EmptyState
                    icon="projects"
                    title="No projects found"
                    description="There are no projects to display a board for."
                    action={isAdmin(user) ? {
                        label: "Create First Project",
                        onClick: () => setShowCreateProject(true)
                    } : undefined}
                />
            ) : (
                <div className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
                    <div className="bg-muted px-6 py-3 border-b">
                        <p className="text-sm text-muted-foreground">
                            {projects.length} project{projects.length !== 1 ? 's' : ''}
                        </p>
                    </div>

                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-border">
                            <thead className="bg-muted">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Project</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Pricing</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Created By</th>
                                    <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="bg-card divide-y divide-border">
                                {projects.map(project => (
                                    <tr key={project.id} className="hover:bg-muted/50">
                                        <td className="px-6 py-4">
                                            <div className="text-sm font-medium text-foreground">{project.name}</div>
                                            <div className="text-sm text-muted-foreground line-clamp-1">{project.description}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">
                                            {formatPricing(project)}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${statusStyles[project.status] || statusStyles.active}`}>
                                                {project.status}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                            {project.created_by_name || '-'}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm">
                                            <button
                                                onClick={() => router.push(`/dashboard/board/${project.id}`)}
                                                className="text-primary hover:text-primary/80 font-medium"
                                            >
                                                Open
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}
        </div>
    )
}
