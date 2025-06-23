import { User } from '@/types/user'
import { Project } from '@/services/projectsService'
import { isAdmin } from '@/utils/permissions'

interface ProjectsGridProps {
    projects: Project[]
    user: User | null
    onEditProject: (project: Project) => void
    onDeleteProject: (projectId: number) => void
    deleting: number | null
}

export default function ProjectsGrid({ projects, user, onEditProject, onDeleteProject, deleting }: ProjectsGridProps) {
    const getStatusColor = (status: string) => {
        switch (status) {
            case 'active': return 'bg-green-100 text-green-800'
            case 'completed': return 'bg-blue-100 text-blue-800'
            case 'on-hold': return 'bg-yellow-100 text-yellow-800'
            default: return 'bg-red-100 text-red-800'
        }
    }

    return (
        <div className="space-y-4">
            <div className="bg-gray-50 px-6 py-3 rounded-lg">
                <p className="text-sm text-gray-600">
                    Showing {projects.length} project{projects.length !== 1 ? 's' : ''}
                </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {projects.map((project) => (
                    <div key={project.id} className="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow">
                        <div className="flex justify-between items-start mb-3">
                            <h3 className="text-lg font-semibold text-gray-900 truncate">
                                {project.name}
                            </h3>
                            <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${getStatusColor(project.status)}`}>
                                {project.status}
                            </span>
                        </div>

                        {project.description && (
                            <p className="text-gray-600 text-sm mb-4 line-clamp-3">
                                {project.description}
                            </p>
                        )}

                        <div className="text-xs text-gray-500 mb-4">
                            <div>Created by: <span className="font-medium">{project.created_by_name}</span></div>
                            <div>Created: <span className="font-medium">{new Date(project.created_at).toLocaleDateString()}</span></div>
                        </div>

                        {isAdmin(user) && (
                            <div className="flex space-x-2 pt-4 border-t">
                                <button
                                    onClick={() => onEditProject(project)}
                                    className="flex-1 text-blue-600 hover:text-blue-800 text-sm font-medium transition-colors"
                                >
                                    Edit
                                </button>
                                <button
                                    onClick={() => onDeleteProject(project.id)}
                                    disabled={deleting === project.id}
                                    className="flex-1 text-red-600 hover:text-red-800 disabled:opacity-50 text-sm font-medium transition-colors"
                                >
                                    {deleting === project.id ? 'Deleting...' : 'Delete'}
                                </button>
                            </div>
                        )}
                    </div>
                ))}
            </div>
        </div>
    )
}