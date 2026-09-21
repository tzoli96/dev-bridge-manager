'use client';

import React from 'react';
import { useRouter } from 'next/navigation';
import { ProjectsService, Project, ProjectClient } from '@/services/projectsService';
import { InvoicesService, Invoice, InvoiceLineItem, InvoiceNoticesService, InvoiceNoticeWithNames, InvoiceReadyEmail } from '@/services/invoicesService';
import { Button } from '@/components/ui/button';
import { Eye, ChevronDown, ChevronUp, ArrowUpRight, Receipt, Mail, Check, Pencil } from 'lucide-react';
import EmailTemplatesModal from '@/components/EmailTemplatesModal';

const paymentStatusInfo: Record<string, { label: string; className: string }> = {
    paid: { label: 'Kifizetve', className: 'bg-success/10 text-success' },
    partially_paid: { label: 'Részben kifizetve', className: 'bg-warning/10 text-warning' },
    expired: { label: 'Lejárt', className: 'bg-destructive/10 text-destructive' },
    outstanding: { label: 'Nyitva', className: 'bg-muted text-muted-foreground' },
    none: { label: 'Nyitva', className: 'bg-muted text-muted-foreground' },
};

interface AutomationRowProps {
    project: Project;
    clients: ProjectClient[];
}

function AutomationRow({ project, clients }: AutomationRowProps) {
    const [enabled, setEnabled] = React.useState(project.auto_invoice_enabled ?? false);
    const [clientId, setClientId] = React.useState<number | null>(project.auto_invoice_client_id ?? null);
    const [autoApprove, setAutoApprove] = React.useState(project.auto_invoice_auto_approve ?? false);
    const [saving, setSaving] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);
    const [showEmailTemplates, setShowEmailTemplates] = React.useState(false);

    const handleToggle = async (checked: boolean) => {
        setError(null);
        if (checked && !clientId) {
            setEnabled(true);
            return;
        }
        try {
            setSaving(true);
            await ProjectsService.updateProject(project.id, {
                auto_invoice_enabled: checked,
                auto_invoice_client_id: checked ? clientId : null,
                auto_invoice_auto_approve: checked ? autoApprove : false,
            });
            setEnabled(checked);
            if (!checked) setAutoApprove(false);
        } catch (err: any) {
            setError(err.message);
        } finally {
            setSaving(false);
        }
    };

    const handleClientChange = async (newClientId: number | null) => {
        setClientId(newClientId);
        if (!enabled) return;
        setError(null);
        try {
            setSaving(true);
            await ProjectsService.updateProject(project.id, {
                auto_invoice_enabled: true,
                auto_invoice_client_id: newClientId,
            });
        } catch (err: any) {
            setError(err.message);
        } finally {
            setSaving(false);
        }
    };

    const handleAutoApproveChange = async (checked: boolean) => {
        setAutoApprove(checked);
        setError(null);
        try {
            setSaving(true);
            await ProjectsService.updateProject(project.id, { auto_invoice_auto_approve: checked });
        } catch (err: any) {
            setAutoApprove(!checked);
            setError(err.message);
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="py-4 border-b border-border last:border-b-0">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
                <div>
                    <p className="text-sm font-medium text-foreground">{project.name}</p>
                    <p className="text-xs text-muted-foreground">{project.pricing_type === 'hourly' ? 'Óradíjas' : 'Fix áras'}</p>
                </div>
                <label className="flex items-center gap-2 text-sm text-foreground flex-shrink-0">
                    <input
                        type="checkbox"
                        checked={enabled}
                        onChange={(e) => handleToggle(e.target.checked)}
                        disabled={saving || clients.length === 0}
                    />
                    Automatikus havi értesítő
                    {saving && <span className="text-xs text-muted-foreground font-normal">(mentés...)</span>}
                </label>
            </div>

            <div className="flex flex-wrap items-end gap-3">
                {clients.length === 0 ? (
                    <p className="text-xs text-muted-foreground">Nincs ügyfél a projekthez rendelve.</p>
                ) : (
                    <>
                        <div className="flex flex-col gap-1">
                            <label className="text-xs text-muted-foreground">Ügyfél</label>
                            <select
                                value={clientId ?? ''}
                                onChange={(e) => handleClientChange(e.target.value ? Number(e.target.value) : null)}
                                className="w-full sm:w-56 px-3 py-1.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={saving}
                            >
                                <option value="">Válasszon ügyfelet...</option>
                                {clients.map((c) => (
                                    <option key={c.client_id} value={c.client_id}>{c.client_name}</option>
                                ))}
                            </select>
                        </div>
                        {enabled && (
                            <div className="flex flex-col gap-1">
                                <label className="text-xs text-muted-foreground">Jóváhagyás módja</label>
                                <select
                                    value={autoApprove ? 'auto' : 'manual'}
                                    onChange={(e) => handleAutoApproveChange(e.target.value === 'auto')}
                                    className="w-full sm:w-48 px-3 py-1.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={saving}
                                >
                                    <option value="manual">Jóváhagyás szükséges</option>
                                    <option value="auto">Automatikus jóváhagyás</option>
                                </select>
                            </div>
                        )}
                    </>
                )}
                <Button
                    variant="ghost"
                    size="sm"
                    icon={Pencil}
                    onClick={() => setShowEmailTemplates(true)}
                >
                    E-mail sablonok
                </Button>
            </div>
            {error && <p className="text-xs text-destructive mt-2">{error}</p>}
            <EmailTemplatesModal
                isOpen={showEmailTemplates}
                onClose={() => setShowEmailTemplates(false)}
                projectId={project.id}
                projectName={project.name}
            />
        </div>
    );
}

export default function BillingPage() {
    const router = useRouter();
    const [projects, setProjects] = React.useState<Project[]>([]);
    const [projectClients, setProjectClients] = React.useState<Record<number, ProjectClient[]>>({});
    const [loadingProjects, setLoadingProjects] = React.useState(true);

    const [selectedProjectId, setSelectedProjectId] = React.useState<number | ''>('');
    const [invoices, setInvoices] = React.useState<Invoice[]>([]);
    const [loadingInvoices, setLoadingInvoices] = React.useState(true);
    const [invoicesError, setInvoicesError] = React.useState<string | null>(null);
    const [viewingPdfId, setViewingPdfId] = React.useState<number | null>(null);
    const [sendingEmailId, setSendingEmailId] = React.useState<number | null>(null);
    const [sendEmailError, setSendEmailError] = React.useState<string | null>(null);
    const [sendEmailSuccessId, setSendEmailSuccessId] = React.useState<number | null>(null);
    const [expandedInvoiceId, setExpandedInvoiceId] = React.useState<number | null>(null);
    const [breakdowns, setBreakdowns] = React.useState<Record<number, InvoiceLineItem[]>>({});
    const [breakdownLoading, setBreakdownLoading] = React.useState<number | null>(null);
    const [emailHistory, setEmailHistory] = React.useState<Record<number, InvoiceReadyEmail[]>>({});
    const [emailHistoryLoading, setEmailHistoryLoading] = React.useState<number | null>(null);

    const [notices, setNotices] = React.useState<InvoiceNoticeWithNames[]>([]);
    const [noticeFilter, setNoticeFilter] = React.useState<'pending' | 'approved' | 'all'>('pending');
    const [loadingNotices, setLoadingNotices] = React.useState(true);
    const [approvingNoticeId, setApprovingNoticeId] = React.useState<number | null>(null);
    const [noticeApprovalError, setNoticeApprovalError] = React.useState<string | null>(null);

    const fetchNotices = React.useCallback(() => {
        setLoadingNotices(true);
        InvoiceNoticesService.listAll(noticeFilter === 'all' ? undefined : noticeFilter)
            .then((res) => setNotices(res.notices || []))
            .catch(() => setNotices([]))
            .finally(() => setLoadingNotices(false));
    }, [noticeFilter]);

    React.useEffect(() => {
        fetchNotices();
    }, [fetchNotices]);

    const handleApproveNotice = async (notice: InvoiceNoticeWithNames) => {
        setNoticeApprovalError(null);
        try {
            setApprovingNoticeId(notice.id);
            const res = await InvoiceNoticesService.approve(notice.project_id, notice.id);
            if (!res.success && !res.notice) {
                setNoticeApprovalError(res.message || 'A jóváhagyás sikertelen');
                return;
            }
            if (res.message) {
                setNoticeApprovalError(res.message);
            }
            fetchNotices();
            InvoicesService.getAllInvoices(selectedProjectId || undefined).then(setInvoices).catch(() => {});
        } catch (err: any) {
            setNoticeApprovalError(err.message);
        } finally {
            setApprovingNoticeId(null);
        }
    };

    React.useEffect(() => {
        ProjectsService.getAllProjects()
            .then(async (allProjects) => {
                setProjects(allProjects);
                const billable = allProjects.filter((p) => p.pricing_type === 'hourly' || p.pricing_type === 'fixed');
                const clientLists = await Promise.all(
                    billable.map((p) => ProjectsService.getProjectClients(p.id).catch(() => []))
                );
                const map: Record<number, ProjectClient[]> = {};
                billable.forEach((p, i) => { map[p.id] = clientLists[i]; });
                setProjectClients(map);
            })
            .finally(() => setLoadingProjects(false));
    }, []);

    React.useEffect(() => {
        setLoadingInvoices(true);
        setInvoicesError(null);
        InvoicesService.getAllInvoices(selectedProjectId || undefined)
            .then(setInvoices)
            .catch((err: any) => setInvoicesError(err.message))
            .finally(() => setLoadingInvoices(false));
    }, [selectedProjectId]);

    const handleViewPdf = async (invoice: Invoice) => {
        try {
            setViewingPdfId(invoice.id);
            const blob = await InvoicesService.downloadInvoicePdf(invoice.project_id, invoice.id);
            const url = URL.createObjectURL(blob);
            window.open(url, '_blank');
            setTimeout(() => URL.revokeObjectURL(url), 60000);
        } catch (error) {
            console.error('Error viewing invoice PDF:', error);
        } finally {
            setViewingPdfId(null);
        }
    };

    const handleSendEmail = async (invoice: Invoice) => {
        setSendEmailError(null);
        try {
            setSendingEmailId(invoice.id);
            await InvoicesService.sendInvoiceEmail(invoice.project_id, invoice.id);
            setSendEmailSuccessId(invoice.id);
            setTimeout(() => setSendEmailSuccessId((current) => (current === invoice.id ? null : current)), 4000);
        } catch (error: any) {
            setSendEmailError(error.message);
        } finally {
            setSendingEmailId(null);
        }
    };

    const toggleBreakdown = async (invoice: Invoice) => {
        if (expandedInvoiceId === invoice.id) {
            setExpandedInvoiceId(null);
            return;
        }
        setExpandedInvoiceId(invoice.id);
        if (!breakdowns[invoice.id]) {
            try {
                setBreakdownLoading(invoice.id);
                const items = await InvoicesService.getInvoiceBreakdown(invoice.project_id, invoice.id);
                setBreakdowns((prev) => ({ ...prev, [invoice.id]: items }));
            } catch (error) {
                console.error('Error loading invoice breakdown:', error);
            } finally {
                setBreakdownLoading(null);
            }
        }
        if (invoice.status === 'created' && !emailHistory[invoice.id]) {
            try {
                setEmailHistoryLoading(invoice.id);
                const emails = await InvoicesService.getInvoiceReadyEmails(invoice.project_id, invoice.id);
                setEmailHistory((prev) => ({ ...prev, [invoice.id]: emails }));
            } catch (error) {
                console.error('Error loading invoice e-mail history:', error);
            } finally {
                setEmailHistoryLoading(null);
            }
        }
    };

    const billableProjects = projects.filter((p) => p.pricing_type === 'hourly' || p.pricing_type === 'fixed');

    return (
        <div className="p-6 max-w-6xl space-y-8">
            <div>
                <h1 className="text-2xl font-bold text-foreground mb-1">Számlázás</h1>
                <p className="text-sm text-muted-foreground">Minden projekt számlái egy helyen, projektenkénti szűréssel.</p>
            </div>

            <div>
                <div className="flex items-center justify-between mb-1">
                    <h2 className="text-lg font-semibold text-foreground">Számla-értesítők</h2>
                    <select
                        value={noticeFilter}
                        onChange={(e) => setNoticeFilter(e.target.value as 'pending' | 'approved' | 'all')}
                        className="px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    >
                        <option value="pending">Jóváhagyásra vár</option>
                        <option value="approved">Jóváhagyva</option>
                        <option value="all">Összes</option>
                    </select>
                </div>
                <p className="text-xs text-muted-foreground mb-3">
                    Ezek az e-mailek a tényleges számla kiállítása előtt mennek ki az ügyfélnek. Jóváhagyás után jön létre a számla, és megy ki a végleges e-mail.
                </p>
                {noticeApprovalError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                        {noticeApprovalError}
                    </div>
                )}
                {loadingNotices ? (
                    <div className="flex items-center justify-center h-20">
                        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                    </div>
                ) : notices.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs megjeleníthető értesítő.</p>
                ) : (
                    <div className="bg-card border border-border rounded-lg divide-y divide-border">
                        {notices.map((notice) => (
                            <div key={notice.id} className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 px-4 py-3">
                                <div className="flex-1 min-w-0">
                                    <p className="text-sm font-medium text-foreground">
                                        {notice.project_name}
                                        {' · '}
                                        {notice.client_name}
                                    </p>
                                    <p className="text-xs text-muted-foreground">
                                        {notice.period_start && notice.period_end ? `${notice.period_start} – ${notice.period_end} · ` : ''}
                                        {'elküldve: '}{new Date(notice.sent_at).toLocaleString('hu-HU')}
                                        {notice.status === 'approved' && notice.billingo_invoice_number ? ` · számla: ${notice.billingo_invoice_number}` : ''}
                                    </p>
                                </div>
                                {notice.status === 'approved' ? (
                                    <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-success/10 text-success flex-shrink-0">
                                        Jóváhagyva
                                    </span>
                                ) : (
                                    <Button
                                        size="sm"
                                        loading={approvingNoticeId === notice.id}
                                        onClick={() => handleApproveNotice(notice)}
                                    >
                                        Jóváhagyás
                                    </Button>
                                )}
                            </div>
                        ))}
                    </div>
                )}
            </div>

            <div>
                <div className="flex items-center justify-between mb-1">
                    <h2 className="text-lg font-semibold text-foreground">Számlák</h2>
                    <select
                        value={selectedProjectId}
                        onChange={(e) => setSelectedProjectId(e.target.value ? Number(e.target.value) : '')}
                        className="px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                        disabled={loadingProjects}
                    >
                        <option value="">Minden projekt</option>
                        {projects.map((p) => (
                            <option key={p.id} value={p.id}>{p.name}</option>
                        ))}
                    </select>
                </div>
                <p className="text-xs text-muted-foreground mb-3">Minden kiállított (vagy sikertelen) számla, projektenkénti szűréssel.</p>

                {invoicesError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                        {invoicesError}
                    </div>
                )}
                {sendEmailError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                        {sendEmailError}
                    </div>
                )}

                {loadingInvoices ? (
                    <div className="flex items-center justify-center h-32">
                        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                    </div>
                ) : invoices.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs még számla.</p>
                ) : (
                    <div className="bg-card border border-border rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="bg-muted/40 text-xs text-muted-foreground text-left">
                                    <th className="px-4 py-2 font-medium">Projekt</th>
                                    <th className="px-4 py-2 font-medium">Ügyfél</th>
                                    <th className="px-4 py-2 font-medium">Tétel</th>
                                    <th className="px-4 py-2 font-medium text-right">Összeg</th>
                                    <th className="px-4 py-2 font-medium">Állapot</th>
                                    <th className="px-4 py-2 font-medium">Esedékesség</th>
                                    <th className="px-4 py-2 font-medium">Kiállítva</th>
                                    <th className="px-4 py-2 font-medium text-center" colSpan={3}>Műveletek</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-border">
                                {invoices.map((invoice) => {
                                    const isHourlyInvoice = invoice.pricing_type === 'hourly';
                                    const extraItems = (invoice.items || []).filter((it) => !it.is_base);
                                    const isExpanded = expandedInvoiceId === invoice.id;
                                    const hasExpandable = isHourlyInvoice || extraItems.length > 0 || invoice.status === 'created';
                                    return (
                                    <React.Fragment key={invoice.id}>
                                    <tr>
                                        <td className="px-4 py-2 text-foreground">{invoice.project_name || `#${invoice.project_id}`}</td>
                                        <td className="px-4 py-2 text-foreground">{invoice.client_name}</td>
                                        <td className="px-4 py-2 text-muted-foreground truncate max-w-[200px]">{invoice.item_name}</td>
                                        <td className="px-4 py-2 text-right font-medium text-foreground whitespace-nowrap">
                                            {invoice.amount.toLocaleString('hu-HU')} HUF
                                        </td>
                                        <td className="px-4 py-2">
                                            <div className="flex flex-wrap items-center gap-1">
                                                <span
                                                    className={[
                                                        'px-2 py-0.5 rounded-full text-[11px] font-medium',
                                                        invoice.status === 'created'
                                                            ? 'bg-success/10 text-success'
                                                            : 'bg-destructive/10 text-destructive',
                                                    ].join(' ')}
                                                >
                                                    {invoice.status === 'created' ? 'Kiállítva' : 'Sikertelen'}
                                                </span>
                                                {invoice.status === 'created' && invoice.payment_status && paymentStatusInfo[invoice.payment_status] && (
                                                    <span
                                                        className={[
                                                            'px-2 py-0.5 rounded-full text-[11px] font-medium',
                                                            paymentStatusInfo[invoice.payment_status].className,
                                                        ].join(' ')}
                                                    >
                                                        {paymentStatusInfo[invoice.payment_status].label}
                                                    </span>
                                                )}
                                            </div>
                                        </td>
                                        <td className="px-4 py-2 text-muted-foreground whitespace-nowrap">
                                            {invoice.due_date ? invoice.due_date.slice(0, 10) : '—'}
                                        </td>
                                        <td className="px-4 py-2 text-muted-foreground whitespace-nowrap">
                                            {invoice.created_at?.slice(0, 10)}
                                        </td>
                                        <td className="px-4 py-2">
                                            {invoice.status === 'created' && invoice.billingo_invoice_id && (
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    icon={Eye}
                                                    loading={viewingPdfId === invoice.id}
                                                    onClick={() => handleViewPdf(invoice)}
                                                    title="Megtekintés"
                                                />
                                            )}
                                        </td>
                                        <td className="px-4 py-2">
                                            {invoice.status === 'created' && invoice.billingo_invoice_id && (
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    icon={sendEmailSuccessId === invoice.id ? Check : Mail}
                                                    loading={sendingEmailId === invoice.id}
                                                    onClick={() => handleSendEmail(invoice)}
                                                    title="E-mail küldése"
                                                />
                                            )}
                                        </td>
                                        <td className="px-4 py-2">
                                            {hasExpandable && (
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    icon={isExpanded ? ChevronUp : ChevronDown}
                                                    onClick={() => toggleBreakdown(invoice)}
                                                    title="Részletek"
                                                />
                                            )}
                                        </td>
                                    </tr>
                                    {isExpanded && (
                                        <tr>
                                            <td colSpan={10} className="px-4 py-4 bg-muted/20 border-t border-border">
                                                <div className="space-y-4">
                                                    {extraItems.length > 0 && (
                                                        <div>
                                                            <div className="text-xs font-medium text-foreground mb-1.5 flex items-center gap-1">
                                                                <Receipt size={12} /> Tételek
                                                            </div>
                                                            <div className="space-y-1">
                                                                {extraItems.map((it) => (
                                                                    <div key={it.id} className="flex items-center justify-between text-xs text-muted-foreground">
                                                                        <span className="truncate">{it.name} ({it.quantity} {it.unit})</span>
                                                                        <span className="flex-shrink-0 text-foreground">{it.line_total.toLocaleString('hu-HU')} HUF</span>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        </div>
                                                    )}

                                                    {isHourlyInvoice && (
                                                        <div>
                                                            <div className="text-xs font-medium text-foreground mb-1.5">Rögzített órák</div>
                                                            {breakdownLoading === invoice.id ? (
                                                                <p className="text-xs text-muted-foreground">Betöltés...</p>
                                                            ) : (breakdowns[invoice.id] || []).length === 0 ? (
                                                                <p className="text-xs text-muted-foreground">Nincs elérhető óra-bontás.</p>
                                                            ) : (
                                                                <div className="space-y-1">
                                                                    {(breakdowns[invoice.id] || []).map((item, i) => (
                                                                        <button
                                                                            key={i}
                                                                            onClick={() => item.board_id && router.push(`/dashboard/board/${invoice.project_id}/${item.board_id}`)}
                                                                            disabled={!item.board_id}
                                                                            className="w-full flex items-center justify-between text-xs text-muted-foreground rounded px-1.5 py-1 -mx-1.5 hover:bg-muted disabled:hover:bg-transparent group text-left"
                                                                        >
                                                                            <span className="truncate flex items-center gap-1">
                                                                                {item.date} · {item.task_title} · {item.user_name}
                                                                                {item.board_id > 0 && (
                                                                                    <ArrowUpRight size={11} className="opacity-0 group-hover:opacity-100 text-primary flex-shrink-0" />
                                                                                )}
                                                                            </span>
                                                                            <span className="flex-shrink-0 text-foreground">{item.hours}h</span>
                                                                        </button>
                                                                    ))}
                                                                </div>
                                                            )}
                                                        </div>
                                                    )}

                                                    {invoice.status === 'created' && (
                                                        <div>
                                                            <div className="text-xs font-medium text-foreground mb-1.5 flex items-center gap-1">
                                                                <Mail size={12} /> E-mail történet
                                                            </div>
                                                            {emailHistoryLoading === invoice.id ? (
                                                                <p className="text-xs text-muted-foreground">Betöltés...</p>
                                                            ) : (emailHistory[invoice.id] || []).length === 0 ? (
                                                                <p className="text-xs text-muted-foreground">Még nem ment ki e-mail ehhez a számlához.</p>
                                                            ) : (
                                                                <div className="space-y-1">
                                                                    {(emailHistory[invoice.id] || []).map((email) => (
                                                                        <div key={email.id} className="flex items-center justify-between text-xs text-muted-foreground">
                                                                            <span className="truncate">Kész számla e-mail → {email.client_name}</span>
                                                                            <span className="flex-shrink-0 text-foreground">
                                                                                {new Date(email.sent_at).toLocaleString('hu-HU')} · {email.sent_by_name}
                                                                            </span>
                                                                        </div>
                                                                    ))}
                                                                </div>
                                                            )}
                                                        </div>
                                                    )}
                                                </div>
                                            </td>
                                        </tr>
                                    )}
                                    </React.Fragment>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>

            <div>
                <h2 className="text-lg font-semibold text-foreground mb-1">Automatikus havi értesítő</h2>
                <p className="text-xs text-muted-foreground mb-3">
                    Projektenként állítható be, hogy induljon-e automatikus havi értesítő, kinek menjen, és kell-e hozzá kézi jóváhagyás.
                </p>
                {loadingProjects ? (
                    <div className="flex items-center justify-center h-32">
                        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                    </div>
                ) : billableProjects.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs árazással rendelkező projekt.</p>
                ) : (
                    <div className="bg-card border border-border rounded-lg px-4">
                        {billableProjects.map((project) => (
                            <AutomationRow key={project.id} project={project} clients={projectClients[project.id] || []} />
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
}
