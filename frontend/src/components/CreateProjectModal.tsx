import { useState, useEffect } from 'react'
import { ProjectsService, ProjectCreateRequest } from '@/services/projectsService'
import { ClientsService, Client } from '@/services/clientsService'

interface CreateProjectModalProps {
    isOpen: boolean
    onClose: () => void
    onSuccess: () => void
}

const initialFormData: ProjectCreateRequest = {
    name: '',
    description: '',
    status: 'active',
    pricing_type: '',
    hourly_rate: null,
    fixed_price: null
}

export default function CreateProjectModal({ isOpen, onClose, onSuccess }: CreateProjectModalProps) {
    const [formData, setFormData] = useState<ProjectCreateRequest>(initialFormData)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [clients, setClients] = useState<Client[]>([])
    const [selectedClientIds, setSelectedClientIds] = useState<number[]>([])

    useEffect(() => {
        if (isOpen) {
            ClientsService.getAllClients().then(setClients).catch(() => setClients([]))
        }
    }, [isOpen])

    const toggleClient = (clientId: number) => {
        setSelectedClientIds(prev =>
            prev.includes(clientId) ? prev.filter(id => id !== clientId) : [...prev, clientId]
        )
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()

        if (!formData.name.trim()) {
            setError('Project name is required')
            return
        }

        if (formData.pricing_type === 'hourly' && (!formData.hourly_rate || formData.hourly_rate <= 0)) {
            setError('Please enter a valid hourly rate')
            return
        }

        if (formData.pricing_type === 'fixed' && (!formData.fixed_price || formData.fixed_price <= 0)) {
            setError('Please enter a valid fixed price')
            return
        }

        try {
            setLoading(true)
            setError(null)

            const project = await ProjectsService.createProject({
                name: formData.name.trim(),
                description: formData.description?.trim() || undefined,
                status: formData.status,
                pricing_type: formData.pricing_type || undefined,
                hourly_rate: formData.pricing_type === 'hourly' ? formData.hourly_rate : undefined,
                fixed_price: formData.pricing_type === 'fixed' ? formData.fixed_price : undefined
            })

            for (const clientId of selectedClientIds) {
                await ProjectsService.assignClientToProject(project.id, clientId)
            }

            // Reset form
            setFormData(initialFormData)
            setSelectedClientIds([])

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
            setFormData(initialFormData)
            setSelectedClientIds([])
            setError(null)
            onClose()
        }
    }

    if (!isOpen) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Create New Project</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    {error && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {error}
                        </div>
                    )}

                    <div>
                        <label htmlFor="name" className="block text-sm font-medium text-foreground mb-1">
                            Project Name *
                        </label>
                        <input
                            type="text"
                            id="name"
                            value={formData.name}
                            onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            placeholder="Enter project name"
                            disabled={loading}
                            required
                        />
                    </div>

                    <div>
                        <label htmlFor="description" className="block text-sm font-medium text-foreground mb-1">
                            Description
                        </label>
                        <textarea
                            id="description"
                            value={formData.description}
                            onChange={(e) => setFormData(prev => ({ ...prev, description: e.target.value }))}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            placeholder="Enter project description"
                            rows={3}
                            disabled={loading}
                        />
                    </div>

                    <div>
                        <label htmlFor="status" className="block text-sm font-medium text-foreground mb-1">
                            Status
                        </label>
                        <select
                            id="status"
                            value={formData.status}
                            onChange={(e) => setFormData(prev => ({
                                ...prev,
                                status: e.target.value as 'active' | 'completed' | 'on-hold' | 'cancelled'
                            }))}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={loading}
                        >
                            <option value="active">Active</option>
                            <option value="completed">Completed</option>
                            <option value="on-hold">On Hold</option>
                            <option value="cancelled">Cancelled</option>
                        </select>
                    </div>

                    <div>
                        <label className="block text-sm font-medium text-foreground mb-1">
                            Pricing
                        </label>
                        <div className="flex gap-4 mb-2">
                            <label className="flex items-center gap-2 text-sm">
                                <input
                                    type="radio"
                                    checked={formData.pricing_type === 'hourly'}
                                    onChange={() => setFormData(prev => ({ ...prev, pricing_type: 'hourly', fixed_price: null }))}
                                    disabled={loading}
                                />
                                Hourly
                            </label>
                            <label className="flex items-center gap-2 text-sm">
                                <input
                                    type="radio"
                                    checked={formData.pricing_type === 'fixed'}
                                    onChange={() => setFormData(prev => ({ ...prev, pricing_type: 'fixed', hourly_rate: null }))}
                                    disabled={loading}
                                />
                                Fixed Price
                            </label>
                            {formData.pricing_type && (
                                <button
                                    type="button"
                                    onClick={() => setFormData(prev => ({ ...prev, pricing_type: '', hourly_rate: null, fixed_price: null }))}
                                    className="text-xs text-muted-foreground hover:text-muted-foreground"
                                    disabled={loading}
                                >
                                    clear
                                </button>
                            )}
                        </div>
                        {formData.pricing_type === 'hourly' && (
                            <input
                                type="number"
                                min="0"
                                step="0.01"
                                value={formData.hourly_rate ?? ''}
                                onChange={(e) => setFormData(prev => ({ ...prev, hourly_rate: e.target.value ? parseFloat(e.target.value) : null }))}
                                placeholder="Hourly rate (HUF)"
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        )}
                        {formData.pricing_type === 'fixed' && (
                            <input
                                type="number"
                                min="0"
                                step="0.01"
                                value={formData.fixed_price ?? ''}
                                onChange={(e) => setFormData(prev => ({ ...prev, fixed_price: e.target.value ? parseFloat(e.target.value) : null }))}
                                placeholder="Fixed price (HUF)"
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        )}
                    </div>

                    {clients.length > 0 && (
                        <div>
                            <label className="block text-sm font-medium text-foreground mb-1">
                                Clients
                            </label>
                            <div className="max-h-32 overflow-y-auto border border-border rounded-lg divide-y">
                                {clients.map(client => (
                                    <label key={client.id} className="flex items-center gap-2 px-3 py-2 text-sm hover:bg-muted">
                                        <input
                                            type="checkbox"
                                            checked={selectedClientIds.includes(client.id)}
                                            onChange={() => toggleClient(client.id)}
                                            disabled={loading}
                                        />
                                        {client.name}
                                    </label>
                                ))}
                            </div>
                        </div>
                    )}

                    <div className="flex space-x-3 pt-4">
                        <button
                            type="button"
                            onClick={handleClose}
                            disabled={loading}
                            className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={loading || !formData.name.trim()}
                            className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                        >
                            {loading ? 'Creating...' : 'Create Project'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}