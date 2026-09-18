'use client'

import { useEffect, useState } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { ClientsService, Client } from '@/services/clientsService'
import { useAuth } from '@/contexts/AuthContext'
import { hasPermission } from '@/utils/permissions'
import RevenueAnalytics from '@/components/RevenueAnalytics'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import { ArrowLeft } from 'lucide-react'

export default function ClientDetailPage() {
    const { clientId } = useParams<{ clientId: string }>()
    const { user } = useAuth()
    const [client, setClient] = useState<Client | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        ClientsService.getClient(Number(clientId))
            .then(setClient)
            .catch((err) => setError(err.message))
            .finally(() => setLoading(false))
    }, [clientId])

    if (loading) return <LoadingState message="Loading client..." />
    if (error || !client) return <ErrorState error={error || 'Client not found'} onRetry={() => window.location.reload()} />

    return (
        <div className="space-y-6">
            <Link href="/dashboard/clients" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
                <ArrowLeft size={14} /> Back to clients
            </Link>

            <div>
                <h1 className="text-2xl font-bold text-foreground">{client.name}</h1>
                <p className="text-muted-foreground text-sm">
                    {client.type === 'individual' ? 'Individual' : 'Company'}
                    {client.tax_number && ` · ${client.tax_number}`}
                </p>
            </div>

            {hasPermission(user, 'invoices.read') && (
                <RevenueAnalytics clientId={client.id} title="Ügyfél bevétel elemzése" />
            )}
        </div>
    )
}
