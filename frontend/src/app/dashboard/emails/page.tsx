// frontend/src/app/dashboard/emails/page.tsx
'use client'

import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { Mail, Send, Paperclip, RefreshCw, LogOut, X, Plus, Sparkles } from 'lucide-react'
import { GmailService, GmailStatus } from '@/services/gmailService'
import { EmailsService, EmailListItem, EmailDetail } from '@/services/emailsService'
import LoadingState from '@/components/ui/LoadingState'
import ComposeEditor, { ComposeEditorHandle } from '@/components/emails/ComposeEditor'

type Folder = 'inbox' | 'sent'

const MAX_ATTACHMENTS = 5
const MAX_ATTACHMENT_SIZE = 10 * 1024 * 1024 // 10MB, mirrors the backend limit

const AVATAR_COLORS = [
    'bg-blue-500', 'bg-emerald-500', 'bg-amber-500', 'bg-rose-500',
    'bg-violet-500', 'bg-cyan-500', 'bg-orange-500', 'bg-teal-500',
]

function avatarColor(seed: string): string {
    let hash = 0
    for (let i = 0; i < seed.length; i++) hash = (hash * 31 + seed.charCodeAt(i)) >>> 0
    return AVATAR_COLORS[hash % AVATAR_COLORS.length]
}

const CATEGORY_STYLES: Record<string, { label: string; className: string }> = {
    ugyfel: { label: 'Ügyfél', className: 'bg-blue-500/10 text-blue-600' },
    szamla: { label: 'Számla', className: 'bg-emerald-500/10 text-emerald-600' },
    marketing: { label: 'Marketing', className: 'bg-amber-500/10 text-amber-600' },
    rendszeruzenet: { label: 'Rendszer', className: 'bg-slate-500/10 text-slate-600' },
    egyeb: { label: 'Egyéb', className: 'bg-muted text-muted-foreground' },
}

function initials(name: string): string {
    const trimmed = name.trim()
    if (!trimmed) return '?'
    const parts = trimmed.split(/\s+/)
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

function formatListDate(iso: string): string {
    const date = new Date(iso)
    const now = new Date()
    const isToday = date.toDateString() === now.toDateString()
    return isToday
        ? date.toLocaleTimeString('hu-HU', { hour: '2-digit', minute: '2-digit' })
        : date.toLocaleDateString('hu-HU', { month: 'short', day: 'numeric' })
}

function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

// Renders email HTML inside a sandboxed iframe so the message's own styling
// (and any embedded CSS/markup) can't bleed into or clash with the app's
// own layout. No "allow-scripts" is granted, so scripts in the email don't run.
function EmailHtmlFrame({ html }: { html: string }) {
    const iframeRef = useRef<HTMLIFrameElement>(null)
    const [height, setHeight] = useState(200)

    return (
        <iframe
            ref={iframeRef}
            srcDoc={html}
            sandbox="allow-same-origin"
            title="E-mail tartalom"
            className="w-full border-0"
            style={{ height }}
            onLoad={() => {
                const doc = iframeRef.current?.contentWindow?.document
                if (doc?.body) {
                    setHeight(doc.body.scrollHeight + 16)
                }
            }}
        />
    )
}

export default function EmailsPage() {
    const searchParams = useSearchParams()
    const [status, setStatus] = useState<GmailStatus | null>(null)
    const [loadingStatus, setLoadingStatus] = useState(true)
    const [connecting, setConnecting] = useState(false)
    const [statusError, setStatusError] = useState<string | null>(null)

    const [folder, setFolder] = useState<Folder>('inbox')
    const [category, setCategory] = useState<string | undefined>(undefined)
    const [emails, setEmails] = useState<EmailListItem[]>([])
    const [emailsPage, setEmailsPage] = useState(1)
    const [emailsTotal, setEmailsTotal] = useState(0)
    const [loadingEmails, setLoadingEmails] = useState(false)
    const [loadingMore, setLoadingMore] = useState(false)
    const [listError, setListError] = useState<string | null>(null)
    const [selected, setSelected] = useState<EmailDetail | null>(null)
    const [selectedId, setSelectedId] = useState<number | null>(null)
    const [detailError, setDetailError] = useState<string | null>(null)

    const [composeOpen, setComposeOpen] = useState(false)
    const [composeTo, setComposeTo] = useState('')
    const [composeSubject, setComposeSubject] = useState('')
    const [replyToId, setReplyToId] = useState<number | undefined>(undefined)
    const [composeFiles, setComposeFiles] = useState<File[]>([])
    const [sending, setSending] = useState(false)
    const [sendError, setSendError] = useState<string | null>(null)
    const [draftingReply, setDraftingReply] = useState(false)
    const [draftError, setDraftError] = useState<string | null>(null)
    const [pendingDraftText, setPendingDraftText] = useState<string | null>(null)
    const [syncing, setSyncing] = useState(false)
    const [syncError, setSyncError] = useState<string | null>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)
    const composeEditorRef = useRef<ComposeEditorHandle>(null)

    useEffect(() => {
        GmailService.getStatus()
            .then(setStatus)
            .catch((err: any) => setStatusError(err.message))
            .finally(() => setLoadingStatus(false))
    }, [])

    const refreshEmails = () => {
        setLoadingEmails(true)
        setListError(null)
        setEmailsPage(1)
        return EmailsService.list(folder, 1, category)
            .then(res => {
                setEmails(res.emails || [])
                setEmailsTotal(res.total || 0)
            })
            .catch((err: any) => setListError(err.message))
            .finally(() => setLoadingEmails(false))
    }

    const loadMoreEmails = () => {
        const nextPage = emailsPage + 1
        setLoadingMore(true)
        setListError(null)
        EmailsService.list(folder, nextPage, category)
            .then(res => {
                setEmails(prev => [...prev, ...(res.emails || [])])
                setEmailsTotal(res.total || 0)
                setEmailsPage(nextPage)
            })
            .catch((err: any) => setListError(err.message))
            .finally(() => setLoadingMore(false))
    }

    useEffect(() => {
        if (!status?.connected) return
        refreshEmails()
    }, [status?.connected, folder, category])

    const handleSync = async () => {
        setSyncing(true)
        setSyncError(null)
        try {
            const res = await GmailService.sync()
            if (!res.success) {
                setSyncError(res.message || 'Szinkronizálás sikertelen')
                return
            }
            setStatus(prev => (prev ? { ...prev, last_synced_at: res.last_synced_at || prev.last_synced_at } : prev))
            await refreshEmails()
        } catch (err: any) {
            setSyncError(err.message)
        } finally {
            setSyncing(false)
        }
    }

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
        setComposeFiles([])
        if (reply) {
            setComposeTo(reply.from || '')
            setComposeSubject(reply.subject?.startsWith('Re:') ? reply.subject : `Re: ${reply.subject || ''}`)
            setReplyToId(reply.id)
        } else {
            setComposeTo('')
            setComposeSubject('')
            setReplyToId(undefined)
        }
        setComposeOpen(true)
    }

    const handleDraftReply = async () => {
        if (!selected?.id) return
        setDraftError(null)
        openCompose(selected)
        try {
            setDraftingReply(true)
            const res = await EmailsService.draftReply(selected.id)
            if (!res.success || !res.draft) {
                setDraftError(res.message || 'Nem sikerült javaslatot generálni.')
                return
            }
            setPendingDraftText(res.draft)
        } catch (err: any) {
            setDraftError(err.message)
        } finally {
            setDraftingReply(false)
        }
    }

    const handleFilesSelected = (files: FileList | null) => {
        if (!files) return
        setSendError(null)
        const incoming = Array.from(files)
        if (composeFiles.length + incoming.length > MAX_ATTACHMENTS) {
            setSendError(`Legfeljebb ${MAX_ATTACHMENTS} fájl csatolható.`)
            return
        }
        const tooLarge = incoming.find(f => f.size > MAX_ATTACHMENT_SIZE)
        if (tooLarge) {
            setSendError(`${tooLarge.name} mérete meghaladja a 10MB-os limitet.`)
            return
        }
        setComposeFiles(prev => [...prev, ...incoming])
    }

    const removeComposeFile = (index: number) => {
        setComposeFiles(prev => prev.filter((_, i) => i !== index))
    }

    const handleSend = async () => {
        setSendError(null)
        if (!composeTo.trim() || !composeSubject.trim()) {
            setSendError('A címzett és a tárgy megadása kötelező.')
            return
        }
        try {
            setSending(true)
            const isEmpty = composeEditorRef.current?.isEmpty() ?? true
            const res = await EmailsService.send({
                to: composeTo.trim(),
                subject: composeSubject.trim(),
                body: isEmpty ? '' : (composeEditorRef.current?.getText() ?? ''),
                body_html: isEmpty ? '' : (composeEditorRef.current?.getHTML() ?? ''),
                in_reply_to_email_id: replyToId,
                files: composeFiles,
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

    useEffect(() => {
        if (composeOpen && pendingDraftText) {
            composeEditorRef.current?.setText(pendingDraftText)
            setPendingDraftText(null)
        }
    }, [composeOpen, pendingDraftText])

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
                    {syncError && (
                        <p className="text-sm text-destructive mt-1">{syncError}</p>
                    )}
                </div>
                <div className="flex gap-2">
                    <button
                        onClick={handleSync}
                        disabled={syncing}
                        className="flex items-center gap-2 px-3 py-2 text-muted-foreground hover:text-foreground rounded-lg transition-colors disabled:opacity-50"
                        title="Szinkronizálás most"
                    >
                        <RefreshCw size={16} className={syncing ? 'animate-spin' : ''} />
                    </button>
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
                                onClick={() => { setFolder(f); setCategory(undefined); setSelected(null); setSelectedId(null) }}
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

                    {folder === 'inbox' && (
                        <div className="flex flex-wrap gap-1.5 px-4 py-2 border-b border-border">
                            <button
                                onClick={() => setCategory(undefined)}
                                className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                                    !category ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground hover:bg-muted/70'
                                }`}
                            >
                                Mind
                            </button>
                            {Object.entries(CATEGORY_STYLES).map(([key, { label }]) => (
                                <button
                                    key={key}
                                    onClick={() => setCategory(key)}
                                    className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                                        category === key ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground hover:bg-muted/70'
                                    }`}
                                >
                                    {label}
                                </button>
                            ))}
                        </div>
                    )}

                    {loadingEmails ? (
                        <div className="p-6"><LoadingState message="E-mailek betöltése..." /></div>
                    ) : listError ? (
                        <p className="p-6 text-sm text-destructive text-center">{listError}</p>
                    ) : emails.length === 0 ? (
                        <div className="p-10 text-center space-y-2">
                            <Mail className="mx-auto text-muted-foreground/40" size={32} />
                            <p className="text-sm text-muted-foreground">
                                {folder === 'inbox' ? 'Nincs beérkezett e-mail.' : 'Nincs elküldött e-mail.'}
                            </p>
                        </div>
                    ) : (
                        <div className="divide-y divide-border max-h-[70vh] overflow-y-auto">
                            {emails.map(item => {
                                const label = folder === 'inbox' ? (item.from_name || item.from_address) : item.to_addresses
                                return (
                                    <button
                                        key={item.id}
                                        onClick={() => openEmail(item)}
                                        className={`w-full text-left px-4 py-3 flex gap-3 hover:bg-muted/50 transition-colors ${
                                            selectedId === item.id ? 'bg-primary/5 border-l-2 border-primary' : 'border-l-2 border-transparent'
                                        }`}
                                    >
                                        <div className={`shrink-0 w-9 h-9 rounded-full flex items-center justify-center text-xs font-semibold text-white ${avatarColor(label)}`}>
                                            {initials(label)}
                                        </div>
                                        <div className="min-w-0 flex-1">
                                            <div className="flex justify-between items-baseline gap-2">
                                                <span className={`text-sm truncate ${!item.is_read ? 'font-semibold text-foreground' : 'text-muted-foreground'}`}>
                                                    {label}
                                                </span>
                                                <span className="text-xs text-muted-foreground shrink-0">
                                                    {formatListDate(item.received_at)}
                                                </span>
                                            </div>
                                            <div className={`text-sm truncate flex items-center gap-1.5 ${!item.is_read ? 'font-medium text-foreground' : 'text-muted-foreground'}`}>
                                                <span className="truncate">{item.subject || '(nincs tárgy)'}</span>
                                                {item.has_attachments && <Paperclip size={12} className="inline shrink-0" />}
                                                {item.category && CATEGORY_STYLES[item.category] && (
                                                    <span className={`shrink-0 px-1.5 py-0.5 rounded-full text-[10px] font-semibold ${CATEGORY_STYLES[item.category].className}`}>
                                                        {CATEGORY_STYLES[item.category].label}
                                                    </span>
                                                )}
                                            </div>
                                            <div className="text-xs text-muted-foreground truncate">{item.snippet}</div>
                                        </div>
                                    </button>
                                )
                            })}
                            {emails.length < emailsTotal && (
                                <button
                                    onClick={loadMoreEmails}
                                    disabled={loadingMore}
                                    className="w-full py-3 text-sm font-medium text-primary hover:bg-muted/50 transition-colors disabled:opacity-50"
                                >
                                    {loadingMore ? 'Betöltés...' : 'Továbbiak betöltése'}
                                </button>
                            )}
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
                            {selected.body_html ? (
                                <EmailHtmlFrame html={selected.body_html} />
                            ) : selected.body_text ? (
                                <div className="prose prose-sm max-w-none text-foreground whitespace-pre-wrap">
                                    {selected.body_text}
                                </div>
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
                            <div className="flex items-center gap-2">
                                <button
                                    onClick={() => openCompose(selected)}
                                    className="flex items-center gap-2 px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 transition-colors"
                                >
                                    <RefreshCw size={14} /> Válasz
                                </button>
                                <button
                                    onClick={handleDraftReply}
                                    disabled={draftingReply}
                                    className="flex items-center gap-2 px-4 py-2 bg-primary/10 text-primary rounded-lg text-sm font-medium hover:bg-primary/20 disabled:opacity-50 transition-colors"
                                >
                                    <Sparkles size={14} /> {draftingReply ? 'Javaslat készül...' : 'AI válasz-javaslat'}
                                </button>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {composeOpen && (
                <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
                    <div className="bg-card rounded-2xl shadow-xl w-full max-w-xl overflow-hidden">
                        <div className="flex justify-between items-center px-6 py-4 border-b border-border">
                            <h3 className="text-lg font-semibold text-foreground">{replyToId ? 'Válasz' : 'Új levél'}</h3>
                            <button onClick={() => setComposeOpen(false)} className="text-muted-foreground hover:text-foreground">
                                <X size={18} />
                            </button>
                        </div>

                        <div className="px-6 py-5 space-y-4">
                            {sendError && (
                                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                                    {sendError}
                                </div>
                            )}
                            {draftError && (
                                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                                    {draftError}
                                </div>
                            )}
                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">Címzett</label>
                                <input
                                    type="email"
                                    placeholder="cimzett@example.com"
                                    value={composeTo}
                                    onChange={e => setComposeTo(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                />
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">Tárgy</label>
                                <input
                                    type="text"
                                    placeholder="Tárgy"
                                    value={composeSubject}
                                    onChange={e => setComposeSubject(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                />
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">Üzenet</label>
                                <ComposeEditor ref={composeEditorRef} />
                            </div>

                            <div className="space-y-2">
                                <input
                                    ref={fileInputRef}
                                    type="file"
                                    multiple
                                    className="hidden"
                                    onChange={e => {
                                        handleFilesSelected(e.target.files)
                                        e.target.value = ''
                                    }}
                                />
                                <button
                                    type="button"
                                    onClick={() => fileInputRef.current?.click()}
                                    disabled={composeFiles.length >= MAX_ATTACHMENTS}
                                    className="flex items-center gap-1.5 text-sm text-primary hover:underline disabled:opacity-50 disabled:no-underline"
                                >
                                    <Plus size={14} /> Csatolmány hozzáadása
                                </button>
                                {composeFiles.length > 0 && (
                                    <ul className="space-y-1.5">
                                        {composeFiles.map((file, i) => (
                                            <li
                                                key={`${file.name}-${i}`}
                                                className="flex items-center justify-between gap-2 bg-muted/60 rounded-lg px-3 py-1.5 text-sm"
                                            >
                                                <span className="flex items-center gap-1.5 min-w-0 truncate text-foreground">
                                                    <Paperclip size={13} className="shrink-0 text-muted-foreground" />
                                                    <span className="truncate">{file.name}</span>
                                                </span>
                                                <span className="flex items-center gap-2 shrink-0">
                                                    <span className="text-xs text-muted-foreground">{formatFileSize(file.size)}</span>
                                                    <button
                                                        type="button"
                                                        onClick={() => removeComposeFile(i)}
                                                        className="text-muted-foreground hover:text-destructive"
                                                    >
                                                        <X size={14} />
                                                    </button>
                                                </span>
                                            </li>
                                        ))}
                                    </ul>
                                )}
                            </div>
                        </div>

                        <div className="flex justify-end gap-3 px-6 py-4 border-t border-border bg-muted/30">
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
