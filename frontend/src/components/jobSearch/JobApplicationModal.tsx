// frontend/src/components/jobSearch/JobApplicationModal.tsx
'use client'

import { useEffect, useState } from 'react'
import { Modal } from '@/components/ui/modal'
import { Button } from '@/components/ui/button'
import { JobSearchService, JobMatch } from '@/services/jobSearchService'
import { EmailsService } from '@/services/emailsService'

interface JobApplicationModalProps {
    match: JobMatch
    onClose: () => void
    onApplied: (matchId: number) => void
}

export function JobApplicationModal({ match, onClose, onApplied }: JobApplicationModalProps) {
    const [draft, setDraft] = useState('')
    const [draftLoading, setDraftLoading] = useState(true)
    const [draftError, setDraftError] = useState<string | null>(null)

    const [email, setEmail] = useState('')
    const [sending, setSending] = useState(false)
    const [sendError, setSendError] = useState<string | null>(null)

    const [markingApplied, setMarkingApplied] = useState(false)

    useEffect(() => {
        setDraftLoading(true)
        setDraftError(null)
        JobSearchService.draftApplication(match.id)
            .then(setDraft)
            .catch((err) => setDraftError(err instanceof Error ? err.message : 'Hiba történt a jelentkezés-tervezet készítése közben.'))
            .finally(() => setDraftLoading(false))
    }, [match.id])

    const handleSendEmail = async () => {
        if (!email.trim()) return
        setSending(true)
        setSendError(null)
        try {
            const sendResult = await EmailsService.send({
                to: email.trim(),
                subject: `Jelentkezés: ${match.job_listing.title}`,
                body: draft,
            })
            if (!sendResult.success) {
                setSendError(sendResult.message || 'Hiba történt az e-mail küldése közben.')
                return
            }
            await JobSearchService.markApplied(match.id, draft)
            onApplied(match.id)
        } catch (err) {
            setSendError(err instanceof Error ? err.message : 'Hiba történt az e-mail küldése közben.')
        } finally {
            setSending(false)
        }
    }

    const handleMarkApplied = async () => {
        setMarkingApplied(true)
        setSendError(null)
        try {
            await JobSearchService.markApplied(match.id, draft)
            onApplied(match.id)
        } catch (err) {
            setSendError(err instanceof Error ? err.message : 'Hiba történt a megjelölés közben.')
        } finally {
            setMarkingApplied(false)
        }
    }

    return (
        <Modal isOpen onClose={onClose} title={`Jelentkezés: ${match.job_listing.title}`} size="lg">
            <div className="p-6 pt-0 space-y-4">
                {draftError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                        {draftError}
                    </div>
                )}
                {sendError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                        {sendError}
                    </div>
                )}

                {draftLoading ? (
                    <div className="text-sm text-muted-foreground py-2">AI tervezet készítése...</div>
                ) : (
                    <textarea
                        value={draft}
                        onChange={(e) => setDraft(e.target.value)}
                        rows={12}
                        className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    />
                )}

                <div>
                    <label htmlFor="apply_email" className="block text-sm font-medium text-foreground mb-1">Címzett e-mail címe</label>
                    <input
                        id="apply_email"
                        type="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        placeholder="hr@ceg.hu"
                        className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    />
                </div>

                <div className="flex flex-wrap gap-3 pt-2">
                    <Button type="button" onClick={handleSendEmail} loading={sending} disabled={!email.trim() || draftLoading}>
                        Küldés emailben
                    </Button>
                    <a href={match.job_listing.external_url} target="_blank" rel="noopener noreferrer">
                        <Button type="button" variant="outline">Hirdetés megnyitása</Button>
                    </a>
                    <Button type="button" variant="secondary" onClick={handleMarkApplied} loading={markingApplied}>
                        Megjelölés jelentkezettként
                    </Button>
                </div>
            </div>
        </Modal>
    )
}
