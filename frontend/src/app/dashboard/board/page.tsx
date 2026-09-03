'use client'

import { useRouter } from 'next/navigation'
import { useEffect } from 'react'
import { useProjects } from '@/hooks/useProjects'
import { useModals } from '@/components/dashboard/DashboardModals'
import { useAuth } from '@/contexts/AuthContext'
import { isAdmin } from '@/utils/permissions'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import EmptyState from '@/components/ui/EmptyState'

const statusStyles: Record<string, string> = {
    active: 'bg-green-100 text-green-700',
    completed: 'bg-blue-100 text-blue-700',
    'on-hold': 'bg-yellow-100 text-yellow-700',
    cancelled: 'bg-gray-100 text-gray-600',
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
                    <h1 className="text-2xl font-bold text-gray-900">Projects</h1>
                    <p className="text-gray-500 text-sm">Select a project to see its boards</p>
                </div>
                {isAdmin(user) && (
                    <button
                        onClick={() => setShowCreateProject(true)}
                        className="bg-blue-600 text-white px-4 py-2 rounded-lg font-medium text-sm shadow-sm hover:bg-blue-700 hover:shadow-md active:scale-[0.98] transition-all"
                    >
                        New Project
                    </button>
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
                <div className="bg-white shadow-sm border border-gray-200 rounded-xl overflow-hidden">
                    <div className="bg-gray-50 px-6 py-3 border-b">
                        <p className="text-sm text-gray-600">
                            {projects.length} project{projects.length !== 1 ? 's' : ''}
                        </p>
                    </div>

                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-gray-200">
                            <thead className="bg-gray-50">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Project</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Pricing</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Created By</th>
                                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="bg-white divide-y divide-gray-200">
                                {projects.map(project => (
                                    <tr key={project.id} className="hover:bg-gray-50">
                                        <td className="px-6 py-4">
                                            <div className="text-sm font-medium text-gray-900">{project.name}</div>
                                            <div className="text-sm text-gray-500 line-clamp-1">{project.description}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                                            {formatPricing(project)}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${statusStyles[project.status] || statusStyles.active}`}>
                                                {project.status}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                                            {project.created_by_name || '-'}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm">
                                            <button
                                                onClick={() => router.push(`/dashboard/board/${project.id}`)}
                                                className="text-blue-600 hover:text-blue-800 font-medium"
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
