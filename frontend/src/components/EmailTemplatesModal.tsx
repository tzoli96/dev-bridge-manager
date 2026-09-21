// frontend/src/components/EmailTemplatesModal.tsx
import { useState, useEffect } from 'react'
import { EmailTemplatesService, EmailTemplate } from '@/services/invoicesService'

interface EmailTemplatesModalProps {
    isOpen: boolean
    onClose: () => void
    projectId: number
    projectName: string
}

const TABS: { type: 'notice' | 'ready'; label: string }[] = [
    { type: 'notice', label: 'Értesítő' },
    { type: 'ready', label: 'Kész számla' },
]

export default function EmailTemplatesModal({ isOpen, onClose, projectId, projectName }: EmailTemplatesModalProps) {
    const [activeTab, setActiveTab] = useState<'notice' | 'ready'>('notice')
    const [templates, setTemplates] = useState<Record<string, EmailTemplate>>({})
    const [subject, setSubject] = useState('')
    const [body, setBody] = useState('')
    const [fetching, setFetching] = useState(true)
    const [saving, setSaving] = useState(false)
    const [resetting, setResetting] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)

    useEffect(() => {
        if (!isOpen) return
        setFetching(true)
        setError(null)
        EmailTemplatesService.list(projectId)
            .then((list) => {
                const byType: Record<string, EmailTemplate> = {}
                list.forEach((tmpl) => { byType[tmpl.email_type] = tmpl })
                setTemplates(byType)
            })
            .catch((err) => setError(err.message))
            .finally(() => setFetching(false))
    }, [isOpen, projectId])

    useEffect(() => {
        const tmpl = templates[activeTab]
        setSubject(tmpl?.subject || '')
        setBody(tmpl?.body || '')
        setSuccess(false)
        setError(null)
    }, [activeTab, templates])

    const handleClose = () => {
        if (saving || resetting) return
        onClose()
    }

    const handleSave = async () => {
        try {
            setSaving(true)
            setError(null)
            const saved = await EmailTemplatesService.save(projectId, activeTab, subject, body)
            setTemplates((prev) => ({ ...prev, [activeTab]: saved }))
            setSuccess(true)
        } catch (err: any) {
            setError(err.message)
        } finally {
            setSaving(false)
        }
    }

    const handleReset = async () => {
        try {
            setResetting(true)
            setError(null)
            const reset = await EmailTemplatesService.reset(projectId, activeTab)
            setTemplates((prev) => ({ ...prev, [activeTab]: reset }))
            setSuccess(true)
        } catch (err: any) {
            setError(err.message)
        } finally {
            setResetting(false)
        }
    }

    if (!isOpen) return null

    const isCustom = templates[activeTab]?.is_custom ?? false

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-lg">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">E-mail sablonok — {projectName}</h2>
                    <button
                        onClick={handleClose}
                        disabled={saving || resetting}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                <div className="flex border-b border-input mb-4">
                    {TABS.map((tab) => (
                        <button
                            key={tab.type}
                            onClick={() => setActiveTab(tab.type)}
                            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
                                activeTab === tab.type
                                    ? 'border-primary text-primary'
                                    : 'border-transparent text-muted-foreground hover:text-foreground'
                            }`}
                        >
                            {tab.label}
                        </button>
                    ))}
                </div>

                {fetching ? (
                    <div className="text-sm text-muted-foreground py-4">Betöltés...</div>
                ) : (
                    <div className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {error}
                            </div>
                        )}
                        {success && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Mentve
                            </div>
                        )}

                        <div className="text-xs text-muted-foreground">
                            {isCustom ? 'Egyedi sablon van beállítva.' : 'Jelenleg az alapértelmezett szöveg van használatban.'}
                            {' '}Használható helyőrzők: <code>{'{{client_name}}'}</code>, <code>{'{{project_name}}'}</code>
                            {activeTab === 'notice' ? (
                                <> , <code>{'{{period}}'}</code> , <code>{'{{draft_summary}}'}</code></>
                            ) : (
                                <> , <code>{'{{invoice_number}}'}</code></>
                            )}
                        </div>

                        <div>
                            <label htmlFor="email_subject" className="block text-sm font-medium text-foreground mb-1">
                                Tárgy
                            </label>
                            <input
                                type="text"
                                id="email_subject"
                                value={subject}
                                onChange={(e) => setSubject(e.target.value)}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={saving || resetting}
                            />
                        </div>

                        <div>
                            <label htmlFor="email_body" className="block text-sm font-medium text-foreground mb-1">
                                Szöveg
                            </label>
                            <textarea
                                id="email_body"
                                value={body}
                                onChange={(e) => setBody(e.target.value)}
                                rows={8}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring font-mono text-sm"
                                disabled={saving || resetting}
                            />
                        </div>

                        <div className="flex space-x-3 pt-2">
                            <button
                                type="button"
                                onClick={handleReset}
                                disabled={saving || resetting || !isCustom}
                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                            >
                                {resetting ? 'Visszaállítás...' : 'Alapértelmezett visszaállítása'}
                            </button>
                            <button
                                type="button"
                                onClick={handleSave}
                                disabled={saving || resetting || !subject.trim() || !body.trim()}
                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                            >
                                {saving ? 'Mentés...' : 'Mentés'}
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}
