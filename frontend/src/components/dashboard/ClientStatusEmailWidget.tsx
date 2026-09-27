'use client'

import React from 'react'
import { ClientStatusEmailService, ClientStatusEmailWithNames } from '@/services/clientStatusEmailService'
import { Button } from '@/components/ui/button'

export default function ClientStatusEmailWidget() {
    const [emails, setEmails] = React.useState<ClientStatusEmailWithNames[]>([])
    const [drafts, setDrafts] = React.useState<Record<number, { subject: string; body: string }>>({})
    const [loading, setLoading] = React.useState(true)
    const [actingId, setActingId] = React.useState<number | null>(null)
    const [error, setError] = React.useState<string | null>(null)

    const fetchEmails = React.useCallback(() => {
        setLoading(true)
        ClientStatusEmailService.list('pending')
            .then((fetched) => {
                setEmails(fetched)
                setDrafts(
                    Object.fromEntries(fetched.map((email) => [email.id, { subject: email.subject, body: email.body }]))
                )
            })
            .catch(() => setEmails([]))
            .finally(() => setLoading(false))
    }, [])

    React.useEffect(() => {
        fetchEmails()
    }, [fetchEmails])

    const handleApprove = async (email: ClientStatusEmailWithNames) => {
        setError(null)
        const draft = drafts[email.id] || { subject: email.subject, body: email.body }
        try {
            setActingId(email.id)
            await ClientStatusEmailService.approve(email.id, draft.subject, draft.body)
            fetchEmails()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Az elküldés sikertelen')
        } finally {
            setActingId(null)
        }
    }

    const handleDismiss = async (email: ClientStatusEmailWithNames) => {
        setError(null)
        try {
            setActingId(email.id)
            await ClientStatusEmailService.dismiss(email.id)
            fetchEmails()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Az elvetés sikertelen')
        } finally {
            setActingId(null)
        }
    }

    if (!loading && emails.length === 0) {
        return null
    }

    return (
        <div className="bg-card rounded-xl shadow-sm border border-border p-6">
            <h2 className="text-lg font-semibold text-foreground mb-1">Heti ügyfél státusz-emailek</h2>
            <p className="text-xs text-muted-foreground mb-3">
                AI által megírt heti összefoglalók, ellenőrzésre és jóváhagyásra várnak. Küldés előtt szabadon szerkeszthetők.
            </p>
            {error && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                    {error}
                </div>
            )}
            {loading ? (
                <div className="flex items-center justify-center h-20">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                </div>
            ) : (
                <div className="space-y-4">
                    {emails.map((email) => {
                        const draft = drafts[email.id] || { subject: email.subject, body: email.body }
                        return (
                            <div key={email.id} className="bg-background border border-border rounded-lg p-4 space-y-3">
                                <div>
                                    <p className="text-sm font-medium text-foreground">
                                        {email.client_name} — {email.period_start} - {email.period_end}
                                    </p>
                                </div>
                                <div>
                                    <label className="block text-xs font-medium text-muted-foreground mb-1">Tárgy</label>
                                    <input
                                        type="text"
                                        value={draft.subject}
                                        onChange={(e) =>
                                            setDrafts((prev) => ({ ...prev, [email.id]: { ...draft, subject: e.target.value } }))
                                        }
                                        className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring text-sm"
                                    />
                                </div>
                                <div>
                                    <label className="block text-xs font-medium text-muted-foreground mb-1">Szöveg</label>
                                    <textarea
                                        value={draft.body}
                                        onChange={(e) =>
                                            setDrafts((prev) => ({ ...prev, [email.id]: { ...draft, body: e.target.value } }))
                                        }
                                        rows={6}
                                        className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring text-sm"
                                    />
                                </div>
                                <div className="flex justify-end gap-2">
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        loading={actingId === email.id}
                                        onClick={() => handleDismiss(email)}
                                    >
                                        Elvetés
                                    </Button>
                                    <Button size="sm" loading={actingId === email.id} onClick={() => handleApprove(email)}>
                                        Elküldés
                                    </Button>
                                </div>
                            </div>
                        )
                    })}
                </div>
            )}
        </div>
    )
}
