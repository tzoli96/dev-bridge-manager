// frontend/src/app/dashboard/emails/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { Mail, Send, Paperclip, RefreshCw, LogOut, X } from 'lucide-react'
import { GmailService, GmailStatus } from '@/services/gmailService'
import { EmailsService, EmailListItem, EmailDetail } from '@/services/emailsService'
import LoadingState from '@/components/ui/LoadingState'

type Folder = 'inbox' | 'sent'

export default function EmailsPage() {
    const searchParams = useSearchParams()
    const [status, setStatus] = useState<GmailStatus | null>(null)
    const [loadingStatus, setLoadingStatus] = useState(true)
    const [connecting, setConnecting] = useState(false)
    const [statusError, setStatusError] = useState<string | null>(null)

    const [folder, setFolder] = useState<Folder>('inbox')
    const [emails, setEmails] = useState<EmailListItem[]>([])
    const [loadingEmails, setLoadingEmails] = useState(false)
    const [listError, setListError] = useState<string | null>(null)
    const [selected, setSelected] = useState<EmailDetail | null>(null)
    const [selectedId, setSelectedId] = useState<number | null>(null)
    const [detailError, setDetailError] = useState<string | null>(null)

    const [composeOpen, setComposeOpen] = useState(false)
    const [composeTo, setComposeTo] = useState('')
    const [composeSubject, setComposeSubject] = useState('')
    const [composeBody, setComposeBody] = useState('')
    const [replyToId, setReplyToId] = useState<number | undefined>(undefined)
    const [sending, setSending] = useState(false)
    const [sendError, setSendError] = useState<string | null>(null)

    useEffect(() => {
        GmailService.getStatus()
            .then(setStatus)
            .catch((err: any) => setStatusError(err.message))
            .finally(() => setLoadingStatus(false))
    }, [])

    useEffect(() => {
        if (!status?.connected) return
        setLoadingEmails(true)
        setListError(null)
        EmailsService.list(folder)
            .then(res => setEmails(res.emails || []))
            .catch((err: any) => setListError(err.message))
            .finally(() => setLoadingEmails(false))
    }, [status?.connected, folder])

    const handleConnect = async () => {
        setConnecting(true)
        try {
            const url = await GmailService.getAuthURL()
            window.location.href = url
        } catch (err: any) {
            setStatusError(err.message)
            setConnecting(false)
        }
    }

    const handleDisconnect = async () => {
        try {
            await GmailService.disconnect()
            setStatus({ success: true, connected: false })
            setEmails([])
            setSelected(null)
        } catch (err: any) {
            setStatusError(err.message)
        }
    }

    const openEmail = async (item: EmailListItem) => {
        setSelectedId(item.id)
        setDetailError(null)
        try {
            const detail = await EmailsService.get(item.id)
            setSelected(detail)
            setEmails(prev => prev.map(e => (e.id === item.id ? { ...e, is_read: true } : e)))
        } catch (err: any) {
            setDetailError(err.message)
        }
    }

    const openCompose = (reply?: EmailDetail) => {
        setSendError(null)
        if (reply) {
            setComposeTo(reply.from || '')
            setComposeSubject(reply.subject?.startsWith('Re:') ? reply.subject : `Re: ${reply.subject || ''}`)
            setComposeBody('')
            setReplyToId(reply.id)
        } else {
            setComposeTo('')
            setComposeSubject('')
            setComposeBody('')
            setReplyToId(undefined)
        }
        setComposeOpen(true)
    }

    const handleSend = async () => {
        setSendError(null)
        if (!composeTo.trim() || !composeSubject.trim()) {
            setSendError('A címzett és a tárgy megadása kötelező.')
            return
        }
        try {
            setSending(true)
            const res = await EmailsService.send({
                to: composeTo.trim(),
                subject: composeSubject.trim(),
                body: composeBody,
                in_reply_to_email_id: replyToId,
            })
            if (!res.success) {
                setSendError(res.message || 'Küldés sikertelen')
                return
            }
            setComposeOpen(false)
        } catch (err: any) {
            setSendError(err.message)
        } finally {
            setSending(false)
        }
    }

    if (loadingStatus) return <LoadingState message="Gmail állapot betöltése..." />

    if (!status?.connected) {
        return (
            <div className="flex items-center justify-center py-24">
                <div className="bg-card border border-border rounded-2xl shadow-lg p-10 max-w-md text-center space-y-4">
                    <div className="mx-auto w-14 h-14 rounded-full bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center">
                        <Mail className="text-primary" size={28} />
                    </div>
                    <h1 className="text-xl font-semibold text-foreground">Gmail összekapcsolása</h1>
                    <p className="text-sm text-muted-foreground">
                        Kösd össze a Gmail-fiókodat, hogy itt lásd az összes bejövő és elküldött e-mailedet, és
                        közvetlenül innen küldhess számla-értesítőket az ügyfeleknek.
                    </p>
                    {searchParams.get('error') === 'oauth_failed' && (
                        <p className="text-sm text-destructive">Az összekapcsolás sikertelen volt, próbáld újra.</p>
                    )}
                    {statusError && (
                        <p className="text-sm text-destructive">{statusError}</p>
                    )}
                    <button
                        onClick={handleConnect}
                        disabled={connecting}
                        className="w-full px-4 py-2.5 bg-primary text-white rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
                    >
                        {connecting ? 'Átirányítás...' : 'Gmail összekapcsolása'}
                    </button>
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">E-mailek</h1>
                    <p className="text-sm text-muted-foreground">
                        {status.email_address}
                        {status.last_synced_at && ` · Utolsó szinkronizáció: ${new Date(status.last_synced_at).toLocaleString('hu-HU')}`}
                    </p>
                    {status.needs_reauth && (
                        <p className="text-sm text-destructive mt-1">
                            A Gmail hozzáférés lejárt, kösd össze újra a fiókot.
                        </p>
                    )}
                    {statusError && (
                        <p className="text-sm text-destructive mt-1">{statusError}</p>
                    )}
                </div>
                <div className="flex gap-2">
                    <button
                        onClick={() => openCompose()}
                        className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg font-medium hover:bg-primary/90 transition-colors"
                    >
                        <Send size={16} /> Új levél
                    </button>
                    <button
                        onClick={handleDisconnect}
                        className="flex items-center gap-2 px-3 py-2 text-muted-foreground hover:text-destructive rounded-lg transition-colors"
                        title="Gmail leválasztása"
                    >
                        <LogOut size={16} />
                    </button>
                </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-5 gap-6">
                <div className="md:col-span-2 bg-card border border-border rounded-xl shadow-sm overflow-hidden">
                    <div className="flex border-b border-border">
                        {(['inbox', 'sent'] as Folder[]).map(f => (
                            <button
                                key={f}
                                onClick={() => { setFolder(f); setSelected(null); setSelectedId(null) }}
                                className={`flex-1 py-3 text-sm font-medium transition-colors ${
                                    folder === f
                                        ? 'text-primary border-b-2 border-primary bg-primary/5'
                                        : 'text-muted-foreground hover:text-foreground'
                                }`}
                            >
                                {f === 'inbox' ? 'Beérkezett' : 'Elküldött'}
                            </button>
                        ))}
                    </div>

                    {loadingEmails ? (
                        <div className="p-6"><LoadingState message="E-mailek betöltése..." /></div>
                    ) : listError ? (
                        <p className="p-6 text-sm text-destructive text-center">{listError}</p>
                    ) : emails.length === 0 ? (
                        <p className="p-6 text-sm text-muted-foreground text-center">Nincs megjeleníthető e-mail.</p>
                    ) : (
                        <div className="divide-y divide-border max-h-[70vh] overflow-y-auto">
                            {emails.map(item => (
                                <button
                                    key={item.id}
                                    onClick={() => openEmail(item)}
                                    className={`w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors ${
                                        selectedId === item.id ? 'bg-primary/5' : ''
                                    }`}
                                >
                                    <div className="flex justify-between items-baseline gap-2">
                                        <span className={`text-sm truncate ${!item.is_read ? 'font-semibold text-foreground' : 'text-muted-foreground'}`}>
                                            {folder === 'inbox' ? (item.from_name || item.from_address) : item.to_addresses}
                                        </span>
                                        <span className="text-xs text-muted-foreground shrink-0">
                                            {new Date(item.received_at).toLocaleDateString('hu-HU')}
                                        </span>
                                    </div>
                                    <div className={`text-sm truncate ${!item.is_read ? 'font-medium text-foreground' : 'text-muted-foreground'}`}>
                                        {item.subject || '(nincs tárgy)'}
                                        {item.has_attachments && <Paperclip size={12} className="inline ml-1 align-text-top" />}
                                    </div>
                                    <div className="text-xs text-muted-foreground truncate">{item.snippet}</div>
                                </button>
                            ))}
                        </div>
                    )}
                </div>

                <div className="md:col-span-3 bg-card border border-border rounded-xl shadow-sm p-6 min-h-[300px]">
                    {detailError ? (
                        <div className="h-full flex items-center justify-center text-destructive text-sm">
                            {detailError}
                        </div>
                    ) : !selected ? (
                        <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
                            Válassz egy e-mailt a bal oldali listából.
                        </div>
                    ) : (
                        <div className="space-y-4">
                            <div>
                                <h2 className="text-lg font-semibold text-foreground">{selected.subject || '(nincs tárgy)'}</h2>
                                <p className="text-sm text-muted-foreground">Feladó: {selected.from}</p>
                                <p className="text-sm text-muted-foreground">Címzett: {selected.to}</p>
                            </div>
                            {selected.body_text ? (
                                <div className="prose prose-sm max-w-none text-foreground whitespace-pre-wrap">
                                    {selected.body_text}
                                </div>
                            ) : selected.body_html ? (
                                <div
                                    className="prose prose-sm max-w-none text-foreground"
                                    dangerouslySetInnerHTML={{ __html: selected.body_html }}
                                />
                            ) : (
                                <div className="prose prose-sm max-w-none text-foreground whitespace-pre-wrap">(üres törzs)</div>
                            )}
                            {selected.attachments && selected.attachments.length > 0 && (
                                <div className="border-t border-border pt-3 space-y-1">
                                    <p className="text-xs font-medium text-muted-foreground">Csatolmányok</p>
                                    {selected.attachments.map(a => (
                                        <button
                                            key={a.attachment_id}
                                            onClick={() => selected.id && EmailsService.downloadAttachment(selected.id, a.attachment_id, a.filename)}
                                            className="flex items-center gap-2 text-sm text-primary hover:underline"
                                        >
                                            <Paperclip size={14} /> {a.filename}
                                        </button>
                                    ))}
                                </div>
                            )}
                            <button
                                onClick={() => openCompose(selected)}
                                className="flex items-center gap-2 px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 transition-colors"
                            >
                                <RefreshCw size={14} /> Válasz
                            </button>
                        </div>
                    )}
                </div>
            </div>

            {composeOpen && (
                <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50">
                    <div className="bg-card rounded-xl shadow-xl p-6 w-full max-w-lg space-y-4">
                        <div className="flex justify-between items-center">
                            <h3 className="text-lg font-semibold">{replyToId ? 'Válasz' : 'Új levél'}</h3>
                            <button onClick={() => setComposeOpen(false)} className="text-muted-foreground hover:text-foreground">
                                <X size={18} />
                            </button>
                        </div>
                        {sendError && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {sendError}
                            </div>
                        )}
                        <input
                            type="email"
                            placeholder="Címzett"
                            value={composeTo}
                            onChange={e => setComposeTo(e.target.value)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <input
                            type="text"
                            placeholder="Tárgy"
                            value={composeSubject}
                            onChange={e => setComposeSubject(e.target.value)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <textarea
                            placeholder="Üzenet"
                            rows={6}
                            value={composeBody}
                            onChange={e => setComposeBody(e.target.value)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <div className="flex justify-end gap-3">
                            <button onClick={() => setComposeOpen(false)} className="px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted/70">
                                Mégse
                            </button>
                            <button
                                onClick={handleSend}
                                disabled={sending}
                                className="px-4 py-2 bg-primary text-white rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50"
                            >
                                {sending ? 'Küldés...' : 'Küldés'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
