import { useState, useEffect } from 'react'
import { ProjectsService, Project, ProjectUpdateRequest } from '@/services/projectsService'

interface EditProjectModalProps {
    isOpen: boolean
    project: Project | null
    onClose: () => void
    onSuccess: () => void
}

export default function EditProjectModal({ isOpen, project, onClose, onSuccess }: EditProjectModalProps) {
    const [formData, setFormData] = useState<ProjectUpdateRequest>({
        name: '',
        description: '',
        status: 'active'
    })
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    // Reset form when project changes
    useEffect(() => {
        if (project) {
            setFormData({
                name: project.name,
                description: project.description,
                status: project.status
            })
            setError(null)
        }
    }, [project])

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()

        if (!project) return

        if (!formData.name?.trim()) {
            setError('Project name is required')
            return
        }

        try {
            setLoading(true)
            setError(null)

            await ProjectsService.updateProject(project.id, {
                name: formData.name?.trim(),
                description: formData.description?.trim() || '',
                status: formData.status
            })

            onSuccess()
            onClose()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    const handleClose = () => {
        if (!loading) {
            setError(null)
            onClose()
        }
    }

    if (!isOpen || !project) return null

    return (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Edit Project</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-gray-400 hover:text-gray-600 disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    {error && (
                        <div className="bg-red-50 border border-red-200 text-red-600 px-3 py-2 rounded text-sm">
                            {error}
                        </div>
                    )}

                    <div>
                        <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                            Project Name *
                        </label>
                        <input
                            type="text"
                            id="name"
                            value={formData.name || ''}
                            onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                            placeholder="Enter project name"
                            disabled={loading}
                            required
                        />
                    </div>

                    <div>
                        <label htmlFor="description" className="block text-sm font-medium text-gray-700 mb-1">
                            Description
                        </label>
                        <textarea
                            id="description"
                            value={formData.description || ''}
                            onChange={(e) => setFormData(prev => ({ ...prev, description: e.target.value }))}
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                            placeholder="Enter project description"
                            rows={3}
                            disabled={loading}
                        />
                    </div>

                    <div>
                        <label htmlFor="status" className="block text-sm font-medium text-gray-700 mb-1">
                            Status
                        </label>
                        <select
                            id="status"
                            value={formData.status || 'active'}
                            onChange={(e) => setFormData(prev => ({
                                ...prev,
                                status: e.target.value as 'active' | 'completed' | 'on-hold' | 'cancelled'
                            }))}
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                            disabled={loading}
                        >
                            <option value="active">Active</option>
                            <option value="completed">Completed</option>
                            <option value="on-hold">On Hold</option>
                            <option value="cancelled">Cancelled</option>
                        </select>
                    </div>

                    <div className="bg-gray-50 p-3 rounded-md">
                        <div className="text-sm text-gray-600">
                            <div>Created by: <span className="font-medium">{project.created_by_name}</span></div>
                            <div>Created: <span className="font-medium">{new Date(project.created_at).toLocaleDateString()}</span></div>
                        </div>
                    </div>

                    <div className="flex space-x-3 pt-4">
                        <button
                            type="button"
                            onClick={handleClose}
                            disabled={loading}
                            className="flex-1 px-4 py-2 text-gray-700 bg-gray-200 rounded-md hover:bg-gray-300 disabled:opacity-50 transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={loading || !formData.name?.trim()}
                            className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors"
                        >
                            {loading ? 'Updating...' : 'Update Project'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}