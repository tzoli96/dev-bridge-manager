// frontend/src/components/CreateInvoiceModal.tsx
import { useState, useEffect } from 'react'
import { Project, ProjectClient, ProjectsService } from '@/services/projectsService'
import { InvoicesService, Invoice } from '@/services/invoicesService'

interface CreateInvoiceModalProps {
    isOpen: boolean
    project: Project | null
    onClose: () => void
    onSuccess: (invoice: Invoice) => void
}

export default function CreateInvoiceModal({ isOpen, project, onClose, onSuccess }: CreateInvoiceModalProps) {
    const [clients, setClients] = useState<ProjectClient[]>([])
    const [clientsLoading, setClientsLoading] = useState(true)
    const [selectedClientId, setSelectedClientId] = useState<number | null>(null)
    const [periodStart, setPeriodStart] = useState('')
    const [periodEnd, setPeriodEnd] = useState('')
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        if (isOpen && project) {
            setClientsLoading(true)
            ProjectsService.getProjectClients(project.id)
                .then(setClients)
                .catch(() => setClients([]))
                .finally(() => setClientsLoading(false))
        }
    }, [isOpen, project])

    const handleClose = () => {
        if (!loading) {
            setSelectedClientId(null)
            setPeriodStart('')
            setPeriodEnd('')
            setError(null)
            onClose()
        }
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!project) return

        if (!selectedClientId) {
            setError('Please select a client')
            return
        }

        if (project.pricing_type === 'hourly' && (!periodStart || !periodEnd)) {
            setError('Please select a period for hourly billing')
            return
        }

        try {
            setLoading(true)
            setError(null)

            const invoice = await InvoicesService.createInvoice(project.id, {
                client_id: selectedClientId,
                period_start: project.pricing_type === 'hourly' ? periodStart : undefined,
                period_end: project.pricing_type === 'hourly' ? periodEnd : undefined
            })

            onSuccess(invoice)
            handleClose()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    if (!isOpen || !project) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Számla kiállítása</h2>
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
                        <label htmlFor="client" className="block text-sm font-medium text-foreground mb-1">
                            Ügyfél *
                        </label>
                        <select
                            id="client"
                            value={selectedClientId ?? ''}
                            onChange={(e) => setSelectedClientId(e.target.value ? Number(e.target.value) : null)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={loading || clientsLoading}
                            required
                        >
                            <option value="">
                                {clientsLoading ? 'Loading clients...' : 'Válasszon ügyfelet...'}
                            </option>
                            {clients.map((pc) => (
                                <option key={pc.client_id} value={pc.client_id}>
                                    {pc.client_name}
                                </option>
                            ))}
                        </select>
                        {!clientsLoading && clients.length === 0 && (
                            <p className="text-xs text-muted-foreground mt-1">
                                No clients are attached to this project yet.
                            </p>
                        )}
                    </div>

                    {project.pricing_type === 'hourly' && (
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label htmlFor="period_start" className="block text-sm font-medium text-foreground mb-1">
                                    Időszak kezdete *
                                </label>
                                <input
                                    type="date"
                                    id="period_start"
                                    value={periodStart}
                                    onChange={(e) => setPeriodStart(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                    required
                                />
                            </div>
                            <div>
                                <label htmlFor="period_end" className="block text-sm font-medium text-foreground mb-1">
                                    Időszak vége *
                                </label>
                                <input
                                    type="date"
                                    id="period_end"
                                    value={periodEnd}
                                    onChange={(e) => setPeriodEnd(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                    required
                                />
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
                            Mégse
                        </button>
                        <button
                            type="submit"
                            disabled={loading || clientsLoading || clients.length === 0}
                            className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                        >
                            {loading ? 'Kiállítás...' : 'Számla kiállítása'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}
