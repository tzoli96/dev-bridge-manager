'use client';

import React from 'react';
import { useRouter } from 'next/navigation';
import { ProjectsService, Project, ProjectClient } from '@/services/projectsService';
import { InvoicesService, Invoice, InvoiceLineItem } from '@/services/invoicesService';
import { Button } from '@/components/ui/button';
import { Eye, ChevronDown, ChevronUp, ArrowUpRight, Receipt } from 'lucide-react';

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
    const [saving, setSaving] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);

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
            });
            setEnabled(checked);
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

    return (
        <div className="flex flex-col sm:flex-row sm:items-center gap-3 py-3 border-b border-border last:border-b-0">
            <div className="sm:w-56 flex-shrink-0">
                <p className="text-sm font-medium text-foreground">{project.name}</p>
                <p className="text-xs text-muted-foreground">{project.pricing_type === 'hourly' ? 'Óradíjas' : 'Fix áras'}</p>
            </div>
            <label className="flex items-center gap-2 text-sm text-foreground">
                <input
                    type="checkbox"
                    checked={enabled}
                    onChange={(e) => handleToggle(e.target.checked)}
                    disabled={saving || clients.length === 0}
                />
                Automatikus számlázás
                {saving && <span className="text-xs text-muted-foreground font-normal">(mentés...)</span>}
            </label>
            {clients.length === 0 ? (
                <p className="text-xs text-muted-foreground">Nincs ügyfél a projekthez rendelve.</p>
            ) : (
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
            )}
            {error && <p className="text-xs text-destructive">{error}</p>}
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
    const [expandedInvoiceId, setExpandedInvoiceId] = React.useState<number | null>(null);
    const [breakdowns, setBreakdowns] = React.useState<Record<number, InvoiceLineItem[]>>({});
    const [breakdownLoading, setBreakdownLoading] = React.useState<number | null>(null);

    React.useEffect(() => {
        ProjectsService.getAllProjects()
            .then(async (allProjects) => {
                setProjects(allProjects);
                const billable = allProjects.filter((p) => p.pricing_type);
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
    };

    const billableProjects = projects.filter((p) => p.pricing_type);

    return (
        <div className="p-6 max-w-6xl space-y-8">
            <div>
                <h1 className="text-2xl font-bold text-foreground mb-1">Számlázás</h1>
                <p className="text-sm text-muted-foreground">Minden projekt számlái egy helyen, projektenkénti szűréssel.</p>
            </div>

            <div>
                <div className="flex items-center justify-between mb-3">
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

                {invoicesError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                        {invoicesError}
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
                                    <th className="px-4 py-2 font-medium w-10" />
                                    <th className="px-4 py-2 font-medium w-10" />
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-border">
                                {invoices.map((invoice) => {
                                    const isHourlyInvoice = invoice.pricing_type === 'hourly';
                                    const extraItems = (invoice.items || []).filter((it) => !it.is_base);
                                    const isExpanded = expandedInvoiceId === invoice.id;
                                    const hasExpandable = isHourlyInvoice || extraItems.length > 0;
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
                                            <td colSpan={9} className="px-4 py-4 bg-muted/20 border-t border-border">
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
                <h2 className="text-lg font-semibold text-foreground mb-3">Automatikus számlázás</h2>
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
