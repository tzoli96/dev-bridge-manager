'use client'

import React from 'react'
import Link from 'next/link'
import { ClientHealthService, ClientHealth } from '@/services/clientHealthService'

const STATUS_DOT: Record<ClientHealth['status'], string> = {
    red: 'bg-destructive',
    yellow: 'bg-warning',
    green: 'bg-success',
}

function signalsText(client: ClientHealth): string {
    const signals: string[] = []
    if (client.has_stalled_task) signals.push('elakadt task')
    if (client.has_overdue_invoice) signals.push('lejárt számla')
    return signals.join(', ')
}

export default function ClientHealthWidget() {
    const [clients, setClients] = React.useState<ClientHealth[]>([])
    const [loading, setLoading] = React.useState(true)

    React.useEffect(() => {
        ClientHealthService.list()
            .then(setClients)
            .catch(() => setClients([]))
            .finally(() => setLoading(false))
    }, [])

    const atRisk = clients
        .filter(c => c.status !== 'green')
        .sort((a, b) => (a.status === b.status ? 0 : a.status === 'red' ? -1 : 1))

    if (!loading && atRisk.length === 0) {
        return null
    }

    return (
        <div className="bg-card rounded-xl shadow-sm border border-border p-6">
            <h2 className="text-lg font-semibold text-foreground mb-1">Kockázatos ügyfelek</h2>
            <p className="text-xs text-muted-foreground mb-3">
                Ügyfelek, akiknek elakadt taskjuk és/vagy lejárt, ki nem fizetett számlájuk van.
            </p>
            {loading ? (
                <div className="flex items-center justify-center h-20">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                </div>
            ) : (
                <div className="bg-background border border-border rounded-lg divide-y divide-border">
                    {atRisk.map((client) => (
                        <div key={client.client_id} className="flex items-center gap-3 px-4 py-3">
                            <span className={`inline-block h-2.5 w-2.5 rounded-full flex-shrink-0 ${STATUS_DOT[client.status]}`} />
                            <div className="flex-1 min-w-0">
                                <Link
                                    href={`/dashboard/clients/${client.client_id}`}
                                    className="text-sm font-medium text-foreground hover:text-primary hover:underline"
                                >
                                    {client.client_name}
                                </Link>
                                <p className="text-xs text-muted-foreground">{signalsText(client)}</p>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
