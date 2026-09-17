'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { ProjectsService, Project, ProjectClient } from '@/services/projectsService';
import { InvoicesService, Invoice, InvoiceExtraItemInput, InvoiceNoticesService } from '@/services/invoicesService';
import { timeEntryService } from '@/services/kanban';
import type { TimeEntry } from '@/types/kanban';
import { Button } from '@/components/ui/button';
import { ArrowLeft, ChevronLeft, ChevronRight, FileText, Plus, Trash2 } from 'lucide-react';

const WEEKDAY_LABELS = ['H', 'K', 'Sze', 'Cs', 'P', 'Szo', 'V'];
const INVOICE_DUE_DAYS = 8;

interface ExtraItemRow {
    id: number;
    name: string;
    quantity: number;
    unit: string;
    unit_price: number;
}

function toDateKey(date: Date): string {
    return date.toISOString().slice(0, 10);
}

function addDays(dateKey: string, days: number): string {
    const d = new Date(dateKey);
    d.setDate(d.getDate() + days);
    return toDateKey(d);
}

function buildMonthGrid(year: number, month: number): Date[] {
    const firstOfMonth = new Date(year, month, 1);
    // Monday-first grid: shift so Monday = 0 ... Sunday = 6
    const startOffset = (firstOfMonth.getDay() + 6) % 7;
    const gridStart = new Date(year, month, 1 - startOffset);

    const days: Date[] = [];
    for (let i = 0; i < 42; i++) {
        days.push(new Date(gridStart.getFullYear(), gridStart.getMonth(), gridStart.getDate() + i));
    }
    return days;
}

export default function InvoicePreviewPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();

    const [project, setProject] = React.useState<Project | null>(null);
    const [clients, setClients] = React.useState<ProjectClient[]>([]);
    const [entries, setEntries] = React.useState<TimeEntry[]>([]);
    const [invoices, setInvoices] = React.useState<Invoice[]>([]);
    const [isLoading, setIsLoading] = React.useState(true);

    const [monthCursor, setMonthCursor] = React.useState(() => {
        const now = new Date();
        return new Date(now.getFullYear(), now.getMonth(), 1);
    });
    const [periodStart, setPeriodStart] = React.useState('');
    const [periodEnd, setPeriodEnd] = React.useState('');
    const [selectedClientId, setSelectedClientId] = React.useState<number | null>(null);
    const [itemName, setItemName] = React.useState('');
    const [dueDate, setDueDate] = React.useState('');
    const [dueDateTouched, setDueDateTouched] = React.useState(false);
    const [extraItems, setExtraItems] = React.useState<ExtraItemRow[]>([]);
    const nextExtraItemId = React.useRef(1);
    const [baseUnitPriceInput, setBaseUnitPriceInput] = React.useState('');

    const [submitting, setSubmitting] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);

    const [autoInvoiceEnabled, setAutoInvoiceEnabled] = React.useState(false);
    const [autoInvoiceClientId, setAutoInvoiceClientId] = React.useState<number | null>(null);
    const [savingAutoInvoice, setSavingAutoInvoice] = React.useState(false);
    const [autoInvoiceError, setAutoInvoiceError] = React.useState<string | null>(null);

    const [noticeSending, setNoticeSending] = React.useState(false);
    const [noticeSentAt, setNoticeSentAt] = React.useState<string | null>(null);
    const [noticeError, setNoticeError] = React.useState<string | null>(null);

    const addExtraItem = () => {
        setExtraItems((prev) => [
            ...prev,
            { id: nextExtraItemId.current++, name: '', quantity: 1, unit: 'db', unit_price: 0 }
        ]);
    };

    const removeExtraItem = (id: number) => {
        setExtraItems((prev) => prev.filter((item) => item.id !== id));
    };

    const updateExtraItem = (id: number, field: keyof Omit<ExtraItemRow, 'id'>, value: string) => {
        setExtraItems((prev) =>
            prev.map((item) =>
                item.id === id
                    ? { ...item, [field]: field === 'name' || field === 'unit' ? value : Number(value) }
                    : item
            )
        );
    };

    const handleClientChange = (clientId: number | null) => {
        setSelectedClientId(clientId);
        if (!dueDateTouched) {
            setDueDate(addDays(toDateKey(new Date()), INVOICE_DUE_DAYS));
        }
    };

    const handleDueDateChange = (value: string) => {
        setDueDate(value);
        setDueDateTouched(true);
    };

    React.useEffect(() => {
        Promise.all([
            ProjectsService.getProject(Number(projectId)),
            ProjectsService.getProjectClients(Number(projectId)),
            timeEntryService.getProjectTimeEntries(projectId),
            InvoicesService.getProjectInvoices(Number(projectId))
        ])
            .then(([p, c, e, inv]) => {
                setProject(p);
                setClients(c);
                setEntries(e);
                setInvoices(inv);
                setAutoInvoiceEnabled(p.auto_invoice_enabled ?? false);
                setAutoInvoiceClientId(p.auto_invoice_client_id ?? null);
            })
            .catch(() => setError('Failed to load project data'))
            .finally(() => setIsLoading(false));
    }, [projectId]);

    const hoursByDate = React.useMemo(() => {
        const map = new Map<string, number>();
        for (const entry of entries) {
            map.set(entry.date, (map.get(entry.date) || 0) + entry.hours);
        }
        return map;
    }, [entries]);

    // Days already covered by a previously issued invoice (hourly projects
    // record the billed period on the invoice; there is no once-only rule
    // for hourly like there is for fixed price, so this is purely advisory).
    const invoicedRanges = React.useMemo(
        () => invoices.filter((inv) => inv.status === 'created' && inv.period_start && inv.period_end),
        [invoices]
    );

    const invoicedDateInfo = React.useMemo(() => {
        const map = new Map<string, Invoice>();
        for (const inv of invoicedRanges) {
            let cursor = new Date(inv.period_start as string);
            const end = new Date(inv.period_end as string);
            while (cursor <= end) {
                map.set(toDateKey(cursor), inv);
                cursor = new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate() + 1);
            }
        }
        return map;
    }, [invoicedRanges]);

    const selectedRangeOverlapsInvoiced = React.useMemo(() => {
        if (!periodStart || !periodEnd) return false;
        return invoicedRanges.some(
            (inv) => (inv.period_start as string) <= periodEnd && (inv.period_end as string) >= periodStart
        );
    }, [invoicedRanges, periodStart, periodEnd]);

    const periodEntries = React.useMemo(() => {
        if (!periodStart || !periodEnd) return [];
        return entries.filter((e) => e.date >= periodStart && e.date <= periodEnd);
    }, [entries, periodStart, periodEnd]);

    const totalHours = periodEntries.reduce((sum, e) => sum + e.hours, 0);
    const isHourly = project?.pricing_type === 'hourly';
    const isFixed = project?.pricing_type === 'fixed';
    const defaultBaseUnitPrice = isHourly ? (project?.hourly_rate || 0) : (project?.fixed_price || 0);
    const baseUnitPriceOverride = baseUnitPriceInput.trim() === '' ? null : Number(baseUnitPriceInput);
    const effectiveBaseUnitPrice = baseUnitPriceOverride !== null && !Number.isNaN(baseUnitPriceOverride)
        ? baseUnitPriceOverride
        : defaultBaseUnitPrice;
    const baseQuantity = isHourly ? totalHours : 1;
    const baseUnit = isHourly ? 'óra' : 'db';
    const previewAmount = baseQuantity * effectiveBaseUnitPrice;
    const suggestedItemName = project
        ? isHourly && periodStart && periodEnd
            ? `${project.name} - ${periodStart} to ${periodEnd}`
            : `${project.name} - fixed price`
        : '';
    const validExtraItems = extraItems.filter((item) => item.name.trim().length > 0);
    const extraItemsTotal = validExtraItems.reduce((sum, item) => sum + item.quantity * item.unit_price, 0);
    const grandTotal = previewAmount + extraItemsTotal;

    const handleDayClick = (dateKey: string) => {
        if (!isHourly) return;
        if (!periodStart || (periodStart && periodEnd)) {
            setPeriodStart(dateKey);
            setPeriodEnd('');
        } else if (dateKey < periodStart) {
            setPeriodStart(dateKey);
        } else {
            setPeriodEnd(dateKey);
        }
    };

    const changeMonth = (delta: number) => {
        setMonthCursor((prev) => new Date(prev.getFullYear(), prev.getMonth() + delta, 1));
    };

    const handleAutoInvoiceToggle = async (enabled: boolean) => {
        if (!project) return;
        setAutoInvoiceError(null);
        if (enabled && !autoInvoiceClientId) {
            setAutoInvoiceEnabled(true);
            return;
        }
        try {
            setSavingAutoInvoice(true);
            await ProjectsService.updateProject(project.id, {
                auto_invoice_enabled: enabled,
                auto_invoice_client_id: enabled ? autoInvoiceClientId : null
            });
            setAutoInvoiceEnabled(enabled);
        } catch (err: any) {
            setAutoInvoiceError(err.message);
        } finally {
            setSavingAutoInvoice(false);
        }
    };

    const handleAutoInvoiceClientChange = async (clientId: number | null) => {
        if (!project) return;
        setAutoInvoiceClientId(clientId);
        if (!autoInvoiceEnabled) return;
        setAutoInvoiceError(null);
        try {
            setSavingAutoInvoice(true);
            await ProjectsService.updateProject(project.id, {
                auto_invoice_enabled: true,
                auto_invoice_client_id: clientId
            });
        } catch (err: any) {
            setAutoInvoiceError(err.message);
        } finally {
            setSavingAutoInvoice(false);
        }
    };

    const handleSendInvoiceNotice = async () => {
        if (!project || !selectedClientId) return;
        setNoticeError(null);
        try {
            setNoticeSending(true);
            const res = await InvoiceNoticesService.send(project.id, {
                client_id: selectedClientId,
                period_start: periodStart || undefined,
                period_end: periodEnd || undefined,
            });
            if (!res.success) {
                setNoticeError(res.message || 'Az értesítő küldése sikertelen');
                return;
            }
            setNoticeSentAt(res.notice?.sent_at || new Date().toISOString());
        } catch (err: any) {
            setNoticeError(err.message);
        } finally {
            setNoticeSending(false);
        }
    };

    const handleSubmit = async () => {
        if (!project) return;
        if (!selectedClientId) {
            setError('Please select a client');
            return;
        }
        if (isHourly && (!periodStart || !periodEnd)) {
            setError('Please select a period for hourly billing');
            return;
        }

        try {
            setSubmitting(true);
            setError(null);
            const extra_items: InvoiceExtraItemInput[] = validExtraItems.map((item) => ({
                name: item.name.trim(),
                quantity: item.quantity,
                unit: item.unit,
                unit_price: item.unit_price
            }));
            await InvoicesService.createInvoice(project.id, {
                client_id: selectedClientId,
                period_start: isHourly ? periodStart : undefined,
                period_end: isHourly ? periodEnd : undefined,
                item_name: itemName.trim() || undefined,
                due_date: dueDate || undefined,
                base_unit_price: baseUnitPriceOverride !== null && !Number.isNaN(baseUnitPriceOverride) ? baseUnitPriceOverride : undefined,
                extra_items: extra_items.length > 0 ? extra_items : undefined
            });
            router.push(`/dashboard/board/${projectId}`);
        } catch (err: any) {
            setError(err.message);
        } finally {
            setSubmitting(false);
        }
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

    const days = buildMonthGrid(monthCursor.getFullYear(), monthCursor.getMonth());
    const currentMonth = monthCursor.getMonth();

    return (
        <div className="p-6 max-w-5xl">
            <Link href={`/dashboard/board/${projectId}`} className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
                <ArrowLeft size={14} /> Back to project
            </Link>

            <h1 className="text-2xl font-bold text-foreground mb-1">Számla kiállítása</h1>
            <p className="text-sm text-muted-foreground mb-6">{project?.name}</p>

            {error && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-4">
                    {error}
                </div>
            )}

            {isHourly && (
                <div className="bg-card border border-border rounded-lg p-4 mb-6">
                    <label className="flex items-center gap-2 text-sm font-medium text-foreground mb-1">
                        <input
                            type="checkbox"
                            checked={autoInvoiceEnabled}
                            onChange={(e) => handleAutoInvoiceToggle(e.target.checked)}
                            disabled={savingAutoInvoice || clients.length === 0}
                        />
                        Automatikus havi számlázás
                        {savingAutoInvoice && <span className="text-xs text-muted-foreground font-normal">(mentés...)</span>}
                    </label>
                    <p className="text-xs text-muted-foreground mb-2">
                        Minden hónap 1-jén automatikusan kiszámlázza az előző havi órákat a kiválasztott ügyfélnek.
                    </p>
                    {clients.length === 0 && (
                        <p className="text-xs text-muted-foreground">Előbb adjon hozzá egy ügyfelet a projekthez.</p>
                    )}
                    {autoInvoiceEnabled && clients.length > 0 && (
                        <select
                            value={autoInvoiceClientId ?? ''}
                            onChange={(e) => handleAutoInvoiceClientChange(e.target.value ? Number(e.target.value) : null)}
                            className="w-full max-w-sm px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={savingAutoInvoice}
                        >
                            <option value="">Válasszon ügyfelet...</option>
                            {clients.map((c) => (
                                <option key={c.client_id} value={c.client_id}>{c.client_name}</option>
                            ))}
                        </select>
                    )}
                    {autoInvoiceError && (
                        <p className="text-xs text-destructive mt-2">{autoInvoiceError}</p>
                    )}
                </div>
            )}

            <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
                <div className="lg:col-span-2 bg-card border border-border rounded-lg p-4 h-fit">
                    <div className="flex items-center justify-between mb-4">
                        <button onClick={() => changeMonth(-1)} className="p-1 rounded hover:bg-muted text-muted-foreground">
                            <ChevronLeft size={18} />
                        </button>
                        <h2 className="font-medium text-foreground">
                            {monthCursor.toLocaleDateString('hu-HU', { year: 'numeric', month: 'long' })}
                        </h2>
                        <button onClick={() => changeMonth(1)} className="p-1 rounded hover:bg-muted text-muted-foreground">
                            <ChevronRight size={18} />
                        </button>
                    </div>

                    <div className="grid grid-cols-7 gap-1 text-center text-xs text-muted-foreground mb-1">
                        {WEEKDAY_LABELS.map((d) => (
                            <div key={d} className="py-1">{d}</div>
                        ))}
                    </div>

                    <div className="grid grid-cols-7 gap-1">
                        {days.map((day) => {
                            const dateKey = toDateKey(day);
                            const hours = hoursByDate.get(dateKey) || 0;
                            const inMonth = day.getMonth() === currentMonth;
                            const inSelectedRange = periodStart && periodEnd && dateKey >= periodStart && dateKey <= periodEnd;
                            const isRangeEndpoint = dateKey === periodStart || dateKey === periodEnd;
                            const invoicedBy = invoicedDateInfo.get(dateKey);

                            return (
                                <button
                                    key={dateKey}
                                    onClick={() => handleDayClick(dateKey)}
                                    disabled={!isHourly}
                                    title={invoicedBy ? `Már kiszámlázva: ${invoicedBy.billingo_invoice_number || invoicedBy.period_start + ' – ' + invoicedBy.period_end}` : undefined}
                                    className={[
                                        'relative aspect-square rounded-md p-1 text-xs flex flex-col items-center justify-center border transition-colors',
                                        inMonth ? 'text-foreground' : 'text-muted-foreground/40',
                                        isRangeEndpoint
                                            ? 'bg-primary text-white border-primary'
                                            : inSelectedRange
                                                ? 'bg-primary/10 border-primary/30'
                                                : invoicedBy
                                                    ? 'bg-warning/10 border-warning/30'
                                                    : hours > 0
                                                        ? 'bg-success/10 border-success/20'
                                                        : 'border-transparent',
                                        isHourly ? 'cursor-pointer hover:border-primary/50' : 'cursor-default'
                                    ].join(' ')}
                                >
                                    <span>{day.getDate()}</span>
                                    {hours > 0 && <span className="text-[10px] font-medium">{hours}h</span>}
                                    {invoicedBy && (
                                        <span className="absolute top-0.5 right-0.5 h-1.5 w-1.5 rounded-full bg-warning" />
                                    )}
                                </button>
                            );
                        })}
                    </div>

                    <div className="flex items-center gap-4 mt-3 text-xs text-muted-foreground">
                        <span className="flex items-center gap-1">
                            <span className="h-2.5 w-2.5 rounded-sm bg-success/20 border border-success/30" /> Rögzített óra
                        </span>
                        <span className="flex items-center gap-1">
                            <span className="h-2.5 w-2.5 rounded-sm bg-warning/20 border border-warning/30" /> Már kiszámlázva
                        </span>
                    </div>

                    {isHourly && (
                        <div className="grid grid-cols-2 gap-4 mt-4">
                            <div>
                                <label className="block text-xs font-medium text-foreground mb-1">Időszak kezdete</label>
                                <input
                                    type="date"
                                    value={periodStart}
                                    onChange={(e) => setPeriodStart(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-foreground mb-1">Időszak vége</label>
                                <input
                                    type="date"
                                    value={periodEnd}
                                    onChange={(e) => setPeriodEnd(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                />
                            </div>
                        </div>
                    )}
                </div>

                <div className="lg:col-span-3 bg-card border border-border rounded-lg shadow-sm overflow-hidden">
                    <div className="bg-muted/40 border-b border-border px-6 py-4 flex items-start justify-between">
                        <div>
                            <h2 className="text-lg font-bold text-foreground tracking-tight">SZÁMLA ELŐNÉZET</h2>
                            <p className="text-xs text-muted-foreground mt-0.5">{project?.name}</p>
                        </div>
                        <FileText size={28} className="text-muted-foreground/40" />
                    </div>

                    <div className="p-6 space-y-6">
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-foreground mb-1">Ügyfél *</label>
                                <select
                                    value={selectedClientId ?? ''}
                                    onChange={(e) => handleClientChange(e.target.value ? Number(e.target.value) : null)}
                                    className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                >
                                    <option value="">Válasszon ügyfelet...</option>
                                    {clients.map((c) => (
                                        <option key={c.client_id} value={c.client_id}>{c.client_name}</option>
                                    ))}
                                </select>
                                {clients.length === 0 && (
                                    <p className="text-xs text-muted-foreground mt-1">No clients are attached to this project yet.</p>
                                )}
                            </div>

                            <div>
                                <label className="block text-xs font-medium text-foreground mb-1">Fizetési határidő</label>
                                <input
                                    type="date"
                                    value={dueDate}
                                    onChange={(e) => handleDueDateChange(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                />
                                <p className="text-xs text-muted-foreground mt-1">
                                    {dueDateTouched ? 'Kézzel beállítva.' : `Ügyfél kiválasztásakor automatikusan kitöltve (+${INVOICE_DUE_DAYS} nap).`}
                                </p>
                            </div>
                        </div>

                        {isHourly && (
                            <div className="flex flex-wrap items-center gap-4 text-xs bg-muted/30 border border-border rounded-lg px-3 py-2">
                                <span className="text-muted-foreground">Időszak: <span className="text-foreground font-medium">{periodStart && periodEnd ? `${periodStart} – ${periodEnd}` : '—'}</span></span>
                                <span className="text-muted-foreground">Összes óra: <span className="text-foreground font-medium">{totalHours}</span></span>
                                <span className="text-muted-foreground">Díj: <span className="text-foreground font-medium">{effectiveBaseUnitPrice.toLocaleString('hu-HU')} HUF/h</span></span>
                            </div>
                        )}

                        {selectedRangeOverlapsInvoiced && (
                            <div className="bg-warning/10 border border-warning/30 text-warning px-3 py-2 rounded text-xs">
                                A kiválasztott időszak átfedésben van egy már kiszámlázott időszakkal.
                            </div>
                        )}

                        <div>
                            <table className="w-full text-sm border border-border rounded-lg overflow-hidden">
                                <thead>
                                    <tr className="bg-muted/40 text-xs text-muted-foreground text-left">
                                        <th className="px-3 py-2 font-medium">Tétel</th>
                                        <th className="px-3 py-2 font-medium w-20 text-right">Menny.</th>
                                        <th className="px-3 py-2 font-medium w-16">Egys.</th>
                                        <th className="px-3 py-2 font-medium w-28 text-right">Egységár</th>
                                        <th className="px-3 py-2 font-medium w-28 text-right">Összesen</th>
                                        <th className="px-2 py-2 w-8" />
                                    </tr>
                                </thead>
                                <tbody className="divide-y divide-border">
                                    <tr>
                                        <td className="px-3 py-2">
                                            <input
                                                type="text"
                                                value={itemName}
                                                onChange={(e) => setItemName(e.target.value)}
                                                placeholder={suggestedItemName}
                                                className="w-full bg-transparent focus:outline-none focus:ring-1 focus:ring-ring rounded px-1 py-0.5"
                                            />
                                        </td>
                                        <td className="px-3 py-2 text-right text-muted-foreground">{baseQuantity}</td>
                                        <td className="px-3 py-2 text-muted-foreground">{baseUnit}</td>
                                        <td className="px-3 py-2">
                                            <input
                                                type="number"
                                                value={baseUnitPriceInput}
                                                onChange={(e) => setBaseUnitPriceInput(e.target.value)}
                                                placeholder={defaultBaseUnitPrice.toString()}
                                                className="w-full bg-transparent text-right focus:outline-none focus:ring-1 focus:ring-ring rounded px-1 py-0.5"
                                            />
                                        </td>
                                        <td className="px-3 py-2 text-right font-medium text-foreground">{previewAmount.toLocaleString('hu-HU')}</td>
                                        <td />
                                    </tr>
                                    {extraItems.map((item) => {
                                        const lineTotal = item.quantity * item.unit_price;
                                        return (
                                            <tr key={item.id}>
                                                <td className="px-3 py-2">
                                                    <input
                                                        type="text"
                                                        value={item.name}
                                                        onChange={(e) => updateExtraItem(item.id, 'name', e.target.value)}
                                                        placeholder="Egyedi tétel neve"
                                                        className="w-full bg-transparent focus:outline-none focus:ring-1 focus:ring-ring rounded px-1 py-0.5"
                                                    />
                                                </td>
                                                <td className="px-3 py-2">
                                                    <input
                                                        type="number"
                                                        value={item.quantity}
                                                        onChange={(e) => updateExtraItem(item.id, 'quantity', e.target.value)}
                                                        className="w-full bg-transparent text-right focus:outline-none focus:ring-1 focus:ring-ring rounded px-1 py-0.5"
                                                    />
                                                </td>
                                                <td className="px-3 py-2">
                                                    <input
                                                        type="text"
                                                        value={item.unit}
                                                        onChange={(e) => updateExtraItem(item.id, 'unit', e.target.value)}
                                                        className="w-full bg-transparent focus:outline-none focus:ring-1 focus:ring-ring rounded px-1 py-0.5"
                                                    />
                                                </td>
                                                <td className="px-3 py-2">
                                                    <input
                                                        type="number"
                                                        value={item.unit_price}
                                                        onChange={(e) => updateExtraItem(item.id, 'unit_price', e.target.value)}
                                                        className="w-full bg-transparent text-right focus:outline-none focus:ring-1 focus:ring-ring rounded px-1 py-0.5"
                                                    />
                                                </td>
                                                <td className="px-3 py-2 text-right font-medium text-foreground whitespace-nowrap">
                                                    {lineTotal.toLocaleString('hu-HU')}
                                                </td>
                                                <td className="px-1">
                                                    <button
                                                        onClick={() => removeExtraItem(item.id)}
                                                        className="p-1 rounded hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
                                                        title="Tétel eltávolítása"
                                                    >
                                                        <Trash2 size={14} />
                                                    </button>
                                                </td>
                                            </tr>
                                        );
                                    })}
                                </tbody>
                            </table>

                            <button
                                onClick={addExtraItem}
                                className="mt-2 inline-flex items-center gap-1 text-xs text-primary hover:underline"
                            >
                                <Plus size={14} /> Tétel hozzáadása
                            </button>
                        </div>

                        <div className="border-t border-border pt-3 flex justify-end">
                            <div className="w-56 space-y-1 text-sm">
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Részösszeg</span>
                                    <span className="text-foreground">{previewAmount.toLocaleString('hu-HU')} HUF</span>
                                </div>
                                {validExtraItems.length > 0 && (
                                    <div className="flex justify-between">
                                        <span className="text-muted-foreground">Egyedi tételek</span>
                                        <span className="text-foreground">{extraItemsTotal.toLocaleString('hu-HU')} HUF</span>
                                    </div>
                                )}
                                <div className="flex justify-between font-semibold text-base pt-2 border-t border-border mt-1">
                                    <span>Végösszeg</span>
                                    <span>{grandTotal.toLocaleString('hu-HU')} HUF</span>
                                </div>
                            </div>
                        </div>

                        <div className="mb-4">
                            <button
                                onClick={handleSendInvoiceNotice}
                                disabled={noticeSending || !selectedClientId}
                                className="px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 disabled:opacity-50 transition-colors"
                            >
                                {noticeSending ? 'Küldés...' : 'Értesítő küldése'}
                            </button>
                            {noticeSentAt && (
                                <p className="text-xs text-muted-foreground mt-1">
                                    Elküldve: {new Date(noticeSentAt).toLocaleString('hu-HU')}
                                </p>
                            )}
                            {noticeError && <p className="text-xs text-destructive mt-1">{noticeError}</p>}
                        </div>

                        <Button
                            icon={FileText}
                            className="w-full"
                            loading={submitting}
                            disabled={!project?.pricing_type || clients.length === 0}
                            onClick={handleSubmit}
                        >
                            Számla kiállítása
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
}
