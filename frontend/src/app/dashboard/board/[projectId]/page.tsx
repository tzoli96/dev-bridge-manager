// frontend/src/app/dashboard/board/[projectId]/page.tsx
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { boardService } from '@/services/kanban';
import { InvoicesService, InvoiceLineItem } from '@/services/invoicesService';
import { useProjectInvoices } from '@/hooks/useProjectInvoices';
import { useProjectTasks } from '@/hooks/useProjectTasks';
import { computeDueTasks } from '@/utils/taskStats';
import { hasPermission } from '@/utils/permissions';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, ArrowLeft, KanbanSquare, FileText, Eye, ChevronDown, ChevronUp, Receipt, ArrowUpRight, Wallet, CalendarClock, AlertOctagon, Repeat } from 'lucide-react';
import type { Board } from '@/types/kanban';
import type { Invoice } from '@/services/invoicesService';
import type { DueTask } from '@/utils/taskStats';

export default function BoardsListPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { user, project, invoices, loading: isLoading } = useProjectInvoices(projectId);
    const { tasks } = useProjectTasks(projectId);
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [newBoardName, setNewBoardName] = React.useState('');
    const [isCreating, setIsCreating] = React.useState(false);
    const [expandedInvoiceId, setExpandedInvoiceId] = React.useState<number | null>(null);
    const [breakdowns, setBreakdowns] = React.useState<Record<number, InvoiceLineItem[]>>({});
    const [breakdownLoading, setBreakdownLoading] = React.useState<number | null>(null);
    const [viewingPdfId, setViewingPdfId] = React.useState<number | null>(null);
    const [isHistoryOpen, setIsHistoryOpen] = React.useState(false);

    React.useEffect(() => {
        boardService.listBoards(projectId).then((data) => setBoards(data));
    }, [projectId]);

    const handleViewPdf = async (invoice: Invoice) => {
        if (!project) return;
        try {
            setViewingPdfId(invoice.id);
            const blob = await InvoicesService.downloadInvoicePdf(project.id, invoice.id);
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
        if (!project) return;
        if (expandedInvoiceId === invoice.id) {
            setExpandedInvoiceId(null);
            return;
        }
        setExpandedInvoiceId(invoice.id);
        if (!breakdowns[invoice.id]) {
            try {
                setBreakdownLoading(invoice.id);
                const items = await InvoicesService.getInvoiceBreakdown(project.id, invoice.id);
                setBreakdowns((prev) => ({ ...prev, [invoice.id]: items }));
            } catch (error) {
                console.error('Error loading invoice breakdown:', error);
            } finally {
                setBreakdownLoading(null);
            }
        }
    };

    const paymentStatusInfo: Record<string, { label: string; className: string }> = {
        paid: { label: 'Kifizetve', className: 'bg-success/10 text-success' },
        partially_paid: { label: 'Részben kifizetve', className: 'bg-warning/10 text-warning' },
        expired: { label: 'Lejárt', className: 'bg-destructive/10 text-destructive' },
        outstanding: { label: 'Nyitva', className: 'bg-muted text-muted-foreground' },
        none: { label: 'Nyitva', className: 'bg-muted text-muted-foreground' },
    };

    const handleCreate = async () => {
        if (!newBoardName.trim()) return;
        setIsCreating(true);
        try {
            const board = await boardService.createBoard(projectId, {
                name: newBoardName.trim(),
                position: boards.length,
            });
            setBoards((prev) => [...prev, board]);
            setNewBoardName('');
        } finally {
            setIsCreating(false);
        }
    };

    const dueTasks = React.useMemo(() => computeDueTasks(tasks), [tasks]);
    const boardNameById = React.useMemo(() => {
        const map = new Map<string, string>();
        for (const board of boards) map.set(String(board.id), board.name);
        return map;
    }, [boards]);

    const renderDueTaskRow = ({ task, dueDateKey, isOverdue }: DueTask) => {
        const boardName = task.boardId ? boardNameById.get(task.boardId) : undefined;
        const content = (
            <>
                <span className="min-w-0 flex-1 truncate text-sm text-foreground">{task.title}</span>
                {boardName && (
                    <span className="text-xs text-muted-foreground flex-shrink-0 hidden sm:inline">{boardName}</span>
                )}
                <span
                    className={[
                        'text-xs font-medium px-2 py-0.5 rounded-full flex-shrink-0',
                        isOverdue ? 'bg-destructive/10 text-destructive' : 'bg-warning/10 text-warning',
                    ].join(' ')}
                >
                    {isOverdue ? `Lejárt: ${dueDateKey}` : dueDateKey}
                </span>
            </>
        );
        return task.boardId ? (
            <button
                key={task.id}
                onClick={() => router.push(`/dashboard/board/${projectId}/${task.boardId}`)}
                className="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-muted text-left"
            >
                {content}
            </button>
        ) : (
            <div key={task.id} className="flex items-center gap-3 px-3 py-2">
                {content}
            </div>
        );
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

    return (
        <div className="p-6">
            <Link href="/dashboard/board" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
                <ArrowLeft size={14} /> Back to projects
            </Link>

            <div className="flex items-start justify-between mb-6">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">Boards</h1>
                    {project?.pricing_type && (
                        <div className="flex items-center gap-2 mt-0.5">
                            <p className="text-sm text-muted-foreground">
                                {project.pricing_type === 'hourly'
                                    ? `Óradíjas — ${project.hourly_rate} HUF/óra`
                                    : `Fix áras — ${project.fixed_price} HUF`}
                            </p>
                            {project.auto_invoice_enabled && (
                                <span
                                    className="inline-flex items-center gap-1 text-xs font-medium text-primary bg-primary/10 rounded-full px-2 py-0.5"
                                    title="Minden hónap 1-jén automatikusan értesítőt küld az előző havi órákról, jóváhagyásra várva."
                                >
                                    <Repeat size={12} /> Automatikus havi értesítő
                                </span>
                            )}
                        </div>
                    )}
                </div>
                <div className="flex items-center gap-2">
                    {hasPermission(user, 'invoices.read') && invoices.length > 0 && (
                        <Button
                            variant="secondary"
                            icon={Wallet}
                            onClick={() => router.push(`/dashboard/board/${projectId}/analytics`)}
                        >
                            Áttekintés
                        </Button>
                    )}
                    {project?.pricing_type && hasPermission(user, 'invoices.create') && (
                        <Button icon={FileText} onClick={() => router.push(`/dashboard/board/${projectId}/invoice`)}>
                            Számla kiállítása
                        </Button>
                    )}
                </div>
            </div>

            {(dueTasks.overdue.length > 0 || dueTasks.upcoming.length > 0) && (
                <div className="bg-card border border-border rounded-lg p-5 mb-6 space-y-5">
                    {dueTasks.overdue.length > 0 && (
                        <div>
                            <h2 className="text-lg font-semibold text-destructive mb-3 flex items-center gap-2">
                                <AlertOctagon size={18} /> Lejárt ügyek
                                <span className="text-xs font-normal bg-destructive/10 text-destructive rounded-full px-2 py-0.5">
                                    {dueTasks.overdue.length}
                                </span>
                            </h2>
                            <div className="space-y-1.5">
                                {dueTasks.overdue.map((dueTask) => renderDueTaskRow(dueTask))}
                            </div>
                        </div>
                    )}
                    {dueTasks.upcoming.length > 0 && (
                        <div>
                            <h2 className="text-lg font-semibold text-foreground mb-3 flex items-center gap-2">
                                <CalendarClock size={18} className="text-primary" /> Hamarosan esedékes
                            </h2>
                            <div className="space-y-1.5">
                                {dueTasks.upcoming.map((dueTask) => renderDueTaskRow(dueTask))}
                            </div>
                        </div>
                    )}
                </div>
            )}

            <div className="bg-card border border-border rounded-lg p-5 mb-6">
                <h2 className="text-lg font-semibold text-foreground mb-4">Táblák</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-4">
                    {boards.map((board) => (
                        <button
                            key={board.id}
                            onClick={() => router.push(`/dashboard/board/${projectId}/${board.id}`)}
                            className="flex items-center gap-3 p-4 bg-card border border-border rounded-lg shadow-sm hover:shadow-sm hover:border-primary/40 transition-all text-left"
                        >
                            <KanbanSquare className="text-primary" size={20} />
                            <span className="font-medium text-foreground">{board.name}</span>
                        </button>
                    ))}
                </div>

                <div className="flex items-center gap-2 max-w-sm">
                    <Input
                        value={newBoardName}
                        onChange={setNewBoardName}
                        placeholder="New board name..."
                        className="flex-1"
                    />
                    <Button icon={Plus} onClick={handleCreate} disabled={!newBoardName.trim()} loading={isCreating}>
                        New board
                    </Button>
                </div>
            </div>

            {hasPermission(user, 'invoices.read') && (
                <div>
                    <button
                        onClick={() => setIsHistoryOpen((prev) => !prev)}
                        className="w-full flex items-center justify-between gap-2 mb-3 group"
                    >
                        <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
                            Számlatörténet
                            {invoices.length > 0 && (
                                <span className="text-xs font-normal text-muted-foreground bg-muted rounded-full px-2 py-0.5">
                                    {invoices.length}
                                </span>
                            )}
                        </h2>
                        {invoices.length > 0 && (
                            isHistoryOpen
                                ? <ChevronUp size={18} className="text-muted-foreground group-hover:text-foreground" />
                                : <ChevronDown size={18} className="text-muted-foreground group-hover:text-foreground" />
                        )}
                    </button>
                    {invoices.length === 0 ? (
                        <p className="text-sm text-muted-foreground">No invoices created yet.</p>
                    ) : isHistoryOpen ? (
                        <>
                            <div className="space-y-3">
                                {invoices.map((invoice) => {
                                    const isHourlyInvoice = invoice.pricing_type === 'hourly';
                                    const extraItems = (invoice.items || []).filter((it) => !it.is_base);
                                    const isExpanded = expandedInvoiceId === invoice.id;
                                    const hasExpandable = isHourlyInvoice || extraItems.length > 0;
                                    return (
                                        <div key={invoice.id} className="bg-card border border-border rounded-lg text-sm overflow-hidden">
                                            <div className="flex items-start justify-between gap-4 p-4">
                                                <div className="min-w-0 flex-1">
                                                    <div className="flex items-center gap-2 mb-1">
                                                        <span className="font-semibold text-foreground">
                                                            {invoice.billingo_invoice_number || `Számla #${invoice.id}`}
                                                        </span>
                                                        <span
                                                            className={[
                                                                'px-2 py-0.5 rounded-full text-[11px] font-medium',
                                                                invoice.status === 'created'
                                                                    ? 'bg-success/10 text-success'
                                                                    : 'bg-destructive/10 text-destructive'
                                                            ].join(' ')}
                                                        >
                                                            {invoice.status === 'created' ? 'Kiállítva' : 'Sikertelen'}
                                                        </span>
                                                        {invoice.status === 'created' && invoice.payment_status && paymentStatusInfo[invoice.payment_status] && (
                                                            <span
                                                                className={[
                                                                    'px-2 py-0.5 rounded-full text-[11px] font-medium',
                                                                    paymentStatusInfo[invoice.payment_status].className
                                                                ].join(' ')}
                                                            >
                                                                {paymentStatusInfo[invoice.payment_status].label}
                                                                {invoice.payment_status === 'paid' && invoice.paid_date && ` · ${invoice.paid_date.slice(0, 10)}`}
                                                            </span>
                                                        )}
                                                    </div>
                                                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-0.5 text-xs text-muted-foreground">
                                                        <span>Ügyfél: <span className="text-foreground">{invoice.client_name}</span></span>
                                                        {invoice.item_name && (
                                                            <span className="truncate">Tétel: <span className="text-foreground">{invoice.item_name}</span></span>
                                                        )}
                                                        {invoice.period_start && invoice.period_end && (
                                                            <span>Időszak: <span className="text-foreground">{invoice.period_start} – {invoice.period_end}</span></span>
                                                        )}
                                                        {invoice.due_date && (
                                                            <span>Fizetési határidő: <span className="text-foreground">{invoice.due_date.slice(0, 10)}</span></span>
                                                        )}
                                                        <span>Kiállítva: <span className="text-foreground">{invoice.created_at?.slice(0, 10)}{invoice.created_by_name && ` · ${invoice.created_by_name}`}</span></span>
                                                        {extraItems.length > 0 && (
                                                            <span>Egyedi tételek: <span className="text-foreground">{extraItems.length}</span></span>
                                                        )}
                                                    </div>
                                                    {invoice.status === 'failed' && invoice.error_message && (
                                                        <p className="text-xs text-destructive mt-1">{invoice.error_message}</p>
                                                    )}
                                                </div>

                                                <div className="flex flex-col items-end gap-2 flex-shrink-0">
                                                    <div className="text-lg font-bold text-foreground whitespace-nowrap">
                                                        {invoice.amount.toLocaleString('hu-HU')} HUF
                                                    </div>
                                                    <div className="flex items-center gap-1">
                                                        {invoice.status === 'created' && invoice.billingo_invoice_id && (
                                                            <Button
                                                                variant="ghost"
                                                                size="sm"
                                                                icon={Eye}
                                                                loading={viewingPdfId === invoice.id}
                                                                onClick={() => handleViewPdf(invoice)}
                                                            >
                                                                Megtekintés
                                                            </Button>
                                                        )}
                                                        {hasExpandable && (
                                                            <Button
                                                                variant="ghost"
                                                                size="icon-sm"
                                                                icon={isExpanded ? ChevronUp : ChevronDown}
                                                                onClick={() => toggleBreakdown(invoice)}
                                                            />
                                                        )}
                                                    </div>
                                                </div>
                                            </div>

                                            {isExpanded && (
                                                <div className="border-t border-border bg-muted/20 p-4 space-y-4">
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
                                                                            onClick={() => item.board_id && router.push(`/dashboard/board/${projectId}/${item.board_id}`)}
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
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        </>
                    ) : null}
                </div>
            )}
        </div>
    );
}
