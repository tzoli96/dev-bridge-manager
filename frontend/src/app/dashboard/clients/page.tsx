'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useClients } from '@/hooks/useClients'
import { ClientsService, Client } from '@/services/clientsService'
import { hasAnyPermission, hasPermission } from '@/utils/permissions'
import { useAuth } from '@/contexts/AuthContext'
import CreateClientModal from '@/components/CreateClientModal'
import EditClientModal from '@/components/EditClientModal'
import { Button } from '@/components/ui/button'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import EmptyState from '@/components/ui/EmptyState'

export default function ClientsPage() {
    const { user } = useAuth()
    const { clients, loading, error, refetch } = useClients()
    const [showCreate, setShowCreate] = useState(false)
    const [showEdit, setShowEdit] = useState(false)
    const [selectedClient, setSelectedClient] = useState<Client | null>(null)
    const [deleting, setDeleting] = useState<number | null>(null)

    const canCreate = hasAnyPermission(user, ['clients.create']) || hasPermission(user, 'clients.create')
    const canUpdate = hasAnyPermission(user, ['clients.update'])
    const canDelete = hasAnyPermission(user, ['clients.delete'])

    const handleEdit = (client: Client) => {
        setSelectedClient(client)
        setShowEdit(true)
    }

    const handleDelete = async (client: Client) => {
        if (!confirm(`Are you sure you want to delete "${client.name}"?`)) return

        try {
            setDeleting(client.id)
            await ClientsService.deleteClient(client.id)
            await refetch()
        } catch (err: any) {
            alert(`Failed to delete client: ${err.message}`)
        } finally {
            setDeleting(null)
        }
    }

    if (loading) return <LoadingState message="Loading clients..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">Clients</h1>
                    <p className="text-muted-foreground text-sm">Manage clients and their billing details</p>
                </div>
                {canCreate && (
                    <Button onClick={() => setShowCreate(true)}>
                        New Client
                    </Button>
                )}
            </div>

            {!clients || clients.length === 0 ? (
                <EmptyState
                    icon="files"
                    title="No clients yet"
                    description="No clients have been added to the system yet."
                    action={canCreate ? {
                        label: "Create First Client",
                        onClick: () => setShowCreate(true)
                    } : undefined}
                />
            ) : (
                <div className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
                    <div className="bg-muted px-6 py-3 border-b">
                        <p className="text-sm text-muted-foreground">
                            {clients.length} client{clients.length !== 1 ? 's' : ''}
                        </p>
                    </div>

                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-border">
                            <thead className="bg-muted">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Client</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Type</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Tax Number</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Contact</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
                                    {(canUpdate || canDelete) && (
                                        <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Actions</th>
                                    )}
                                </tr>
                            </thead>
                            <tbody className="bg-card divide-y divide-border">
                                {clients.map(client => (
                                    <tr key={client.id} className="hover:bg-muted/50">
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <Link href={`/dashboard/clients/${client.id}`} className="text-sm font-medium text-foreground hover:text-primary hover:underline">
                                                {client.name}
                                            </Link>
                                            <div className="text-sm text-muted-foreground">{client.billing_city}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span className="inline-flex px-2 py-1 text-xs font-semibold rounded-full bg-primary/10 text-primary">
                                                {client.type === 'individual' ? 'Individual' : 'Company'}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">
                                            {client.tax_number || '-'}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                            <div>{client.email || '-'}</div>
                                            <div>{client.phone || ''}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${client.is_active ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground'}`}>
                                                {client.is_active ? 'Active' : 'Inactive'}
                                            </span>
                                        </td>
                                        {(canUpdate || canDelete) && (
                                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm space-x-3">
                                                {canUpdate && (
                                                    <button
                                                        onClick={() => handleEdit(client)}
                                                        className="text-primary hover:text-primary/80 font-medium"
                                                    >
                                                        Edit
                                                    </button>
                                                )}
                                                {canDelete && (
                                                    <button
                                                        onClick={() => handleDelete(client)}
                                                        disabled={deleting === client.id}
                                                        className="text-destructive hover:text-destructive/80 font-medium disabled:opacity-50"
                                                    >
                                                        {deleting === client.id ? 'Deleting...' : 'Delete'}
                                                    </button>
                                                )}
                                            </td>
                                        )}
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            <CreateClientModal
                isOpen={showCreate}
                onClose={() => setShowCreate(false)}
                onSuccess={refetch}
            />

            <EditClientModal
                isOpen={showEdit}
                client={selectedClient}
                onClose={() => {
                    setShowEdit(false)
                    setSelectedClient(null)
                }}
                onSuccess={refetch}
            />
        </div>
    )
}
