'use client'

import React from 'react'
import { ActivityDigestService, ActivityDigest } from '@/services/activityDigestService'
import { ListPlus, CheckCircle2, Receipt, Mail, AlertTriangle } from 'lucide-react'

function formatHuf(amount: number): string {
    return `${amount.toLocaleString('hu-HU')} HUF`
}

export default function ActivityDigestWidget() {
    const [digest, setDigest] = React.useState<ActivityDigest | null>(null)
    const [loading, setLoading] = React.useState(true)

    React.useEffect(() => {
        ActivityDigestService.get()
            .then(setDigest)
            .catch(() => setDigest(null))
            .finally(() => setLoading(false))
    }, [])

    if (loading) {
        return (
            <div className="bg-card rounded-xl shadow-sm border border-border p-6">
                <div className="flex items-center justify-center h-16">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                </div>
            </div>
        )
    }

    if (!digest) {
        return null
    }

    const hasAnyActivity =
        digest.new_tasks_count > 0 ||
        digest.completed_tasks_count > 0 ||
        digest.new_invoices_count > 0 ||
        digest.new_client_emails_count > 0 ||
        digest.new_stall_flags_count > 0

    return (
        <div className="bg-card rounded-xl shadow-sm border border-border p-6">
            <h2 className="text-lg font-semibold text-foreground mb-1">Napi tevékenység</h2>
            <p className="text-xs text-muted-foreground mb-4">Az előző nap összefoglalója.</p>

            {!hasAnyActivity ? (
                <p className="text-sm text-muted-foreground">Nem történt tevékenység tegnap.</p>
            ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
                    <div className="bg-background border border-border rounded-lg p-3">
                        <div className="flex items-center gap-1.5 text-muted-foreground mb-1">
                            <ListPlus size={13} />
                            <span className="text-xs">Új feladat</span>
                        </div>
                        <div className="text-lg font-bold text-foreground">{digest.new_tasks_count}</div>
                        {digest.new_tasks.slice(0, 3).map((t) => (
                            <div key={t.id} className="text-xs text-muted-foreground truncate">{t.title}</div>
                        ))}
                    </div>

                    <div className="bg-background border border-border rounded-lg p-3">
                        <div className="flex items-center gap-1.5 text-muted-foreground mb-1">
                            <CheckCircle2 size={13} />
                            <span className="text-xs">Befejezett feladat</span>
                        </div>
                        <div className="text-lg font-bold text-foreground">{digest.completed_tasks_count}</div>
                        {digest.completed_tasks.slice(0, 3).map((t) => (
                            <div key={t.id} className="text-xs text-muted-foreground truncate">{t.title}</div>
                        ))}
                    </div>

                    <div className="bg-background border border-border rounded-lg p-3">
                        <div className="flex items-center gap-1.5 text-muted-foreground mb-1">
                            <Receipt size={13} />
                            <span className="text-xs">Új számla</span>
                        </div>
                        <div className="text-lg font-bold text-foreground">{digest.new_invoices_count}</div>
                        {digest.new_invoices_count > 0 && (
                            <div className="text-xs text-muted-foreground">{formatHuf(digest.new_invoices_total)}</div>
                        )}
                    </div>

                    <div className="bg-background border border-border rounded-lg p-3">
                        <div className="flex items-center gap-1.5 text-muted-foreground mb-1">
                            <Mail size={13} />
                            <span className="text-xs">Ügyfél e-mail</span>
                        </div>
                        <div className="text-lg font-bold text-foreground">{digest.new_client_emails_count}</div>
                    </div>

                    <div className="bg-background border border-border rounded-lg p-3">
                        <div className="flex items-center gap-1.5 text-muted-foreground mb-1">
                            <AlertTriangle size={13} />
                            <span className="text-xs">Elakadás jelzés</span>
                        </div>
                        <div className="text-lg font-bold text-foreground">{digest.new_stall_flags_count}</div>
                    </div>
                </div>
            )}
        </div>
    )
}
