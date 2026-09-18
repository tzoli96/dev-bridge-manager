'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { boardService } from '@/services/kanban';
import { InvoicesService } from '@/services/invoicesService';
import { useProjectInvoices } from '@/hooks/useProjectInvoices';
import { useProjectTasks } from '@/hooks/useProjectTasks';
import { computeInvoiceStats } from '@/utils/invoiceStats';
import { computeTaskStats, computeDueTasks } from '@/utils/taskStats';
import RevenueAnalytics from '@/components/RevenueAnalytics';
import { Button } from '@/components/ui/button';
import { ArrowLeft, RefreshCw, Users, AlertTriangle, AlertOctagon, ListChecks, KanbanSquare } from 'lucide-react';
import type { Board } from '@/types/kanban';

export default function ProjectAnalyticsPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { project, invoices, loading: isLoading, reload } = useProjectInvoices(projectId);
    const { tasks } = useProjectTasks(projectId);
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [isRefreshingPayments, setIsRefreshingPayments] = React.useState(false);

    React.useEffect(() => {
        boardService.listBoards(projectId).then(setBoards);
    }, [projectId]);

    const stats = React.useMemo(() => computeInvoiceStats(invoices), [invoices]);
    const taskStats = React.useMemo(() => computeTaskStats(tasks), [tasks]);
    const dueTasks = React.useMemo(() => computeDueTasks(tasks, 20), [tasks]);
    const boardNameById = React.useMemo(() => {
        const map = new Map<string, string>();
        for (const board of boards) map.set(String(board.id), board.name);
        return map;
    }, [boards]);

    const handleRefreshPaymentStatuses = async () => {
        if (!project) return;
        setIsRefreshingPayments(true);
        try {
            await InvoicesService.refreshPaymentStatuses(project.id);
            reload();
        } catch (error) {
            console.error('Error refreshing payment statuses:', error);
        } finally {
            setIsRefreshingPayments(false);
        }
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

    return (
        <div className="p-6 space-y-6">
            <Link href={`/dashboard/board/${projectId}`} className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
                <ArrowLeft size={14} /> Back to boards
            </Link>

            <h1 className="text-2xl font-bold text-foreground">Áttekintés</h1>

            <div className="bg-card border border-border rounded-lg p-5">
                <h2 className="text-lg font-semibold text-foreground mb-4 flex items-center gap-2">
                    <ListChecks size={18} className="text-primary" /> Feladatok
                </h2>
                {tasks.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs még feladat ehhez a projekthez.</p>
                ) : (
                    <>
                        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-5">
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Összes feladat</div>
                                <div className="text-xl font-bold text-foreground">{taskStats.totalTasks}</div>
                            </div>
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Nyitott</div>
                                <div className="text-xl font-bold text-foreground">{taskStats.openTasksCount}</div>
                            </div>
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Kész</div>
                                <div className="text-xl font-bold text-success">{taskStats.doneTasksCount}</div>
                            </div>
                            {(taskStats.overdueCount > 0 || taskStats.dueSoonCount > 0) && (
                                <div className="bg-muted/30 border border-border rounded-lg p-3">
                                    <div className="text-xs text-muted-foreground flex items-center gap-1"><AlertTriangle size={11} /> Határidők</div>
                                    <div className="text-xs font-medium mt-1 space-y-0.5">
                                        {taskStats.overdueCount > 0 && <div className="text-destructive">{taskStats.overdueCount} lejárt</div>}
                                        {taskStats.dueSoonCount > 0 && <div className="text-warning">{taskStats.dueSoonCount} hamarosan esedékes</div>}
                                    </div>
                                </div>
                            )}
                        </div>

                        {dueTasks.overdue.length > 0 && (
                            <div className="mb-5">
                                <div className="text-xs text-destructive mb-1.5 flex items-center gap-1"><AlertOctagon size={12} /> Lejárt ügyek</div>
                                <div className="space-y-1">
                                    {dueTasks.overdue.map(({ task, dueDateKey }) => {
                                        const boardName = task.boardId ? boardNameById.get(task.boardId) : undefined;
                                        const content = (
                                            <>
                                                <span className="min-w-0 flex-1 truncate text-sm text-foreground">{task.title}</span>
                                                {boardName && (
                                                    <span className="text-xs text-muted-foreground flex-shrink-0 hidden sm:inline">{boardName}</span>
                                                )}
                                                <span className="text-xs font-medium px-2 py-0.5 rounded-full flex-shrink-0 bg-destructive/10 text-destructive">
                                                    {dueDateKey}
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
                                    })}
                                </div>
                            </div>
                        )}

                        {boards.length > 1 && (
                            <div>
                                <div className="text-xs text-muted-foreground mb-1.5 flex items-center gap-1"><KanbanSquare size={12} /> Feladatok tábláként</div>
                                <div className="space-y-1.5">
                                    {boards.map((board) => {
                                        const counts = taskStats.byBoard.get(String(board.id));
                                        if (!counts) return null;
                                        return (
                                            <div key={board.id} className="flex items-center gap-2 text-xs">
                                                <span className="w-28 truncate text-muted-foreground flex-shrink-0">{board.name}</span>
                                                <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden">
                                                    <div
                                                        className="h-full bg-primary/70 rounded-full"
                                                        style={{ width: `${counts.total > 0 ? ((counts.total - counts.open) / counts.total) * 100 : 0}%` }}
                                                    />
                                                </div>
                                                <span className="w-24 text-right text-foreground flex-shrink-0">{counts.total - counts.open} / {counts.total} kész</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            </div>
                        )}
                    </>
                )}
            </div>

            <div className="bg-card border border-border rounded-lg p-5">
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold text-foreground">Bevétel</h2>
                    <Button
                        variant="ghost"
                        size="sm"
                        icon={RefreshCw}
                        loading={isRefreshingPayments}
                        onClick={handleRefreshPaymentStatuses}
                    >
                        Fizetettség frissítése
                    </Button>
                </div>

                {invoices.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs még számla ehhez a projekthez.</p>
                ) : (
                    <>
                        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 mb-5">
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Számlák száma</div>
                                <div className="text-xl font-bold text-foreground">{invoices.length}</div>
                            </div>
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Összes bevétel</div>
                                <div className="text-xl font-bold text-foreground">{stats.createdInvoicesTotal.toLocaleString('hu-HU')} HUF</div>
                            </div>
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Kifizetve</div>
                                <div className="text-xl font-bold text-success">{stats.paidTotal.toLocaleString('hu-HU')} HUF</div>
                            </div>
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Nyitva</div>
                                <div className="text-xl font-bold text-foreground">{stats.outstandingTotal.toLocaleString('hu-HU')} HUF</div>
                            </div>
                            <div className="bg-muted/30 border border-border rounded-lg p-3">
                                <div className="text-xs text-muted-foreground">Átlag / számla</div>
                                <div className="text-xl font-bold text-foreground">{Math.round(stats.avgInvoiceAmount).toLocaleString('hu-HU')} HUF</div>
                            </div>
                            {(stats.failedInvoicesCount > 0 || stats.overdueCount > 0) && (
                                <div className="bg-muted/30 border border-border rounded-lg p-3">
                                    <div className="text-xs text-muted-foreground flex items-center gap-1"><AlertTriangle size={11} /> Figyelmeztetés</div>
                                    <div className="text-xs font-medium text-destructive mt-1 space-y-0.5">
                                        {stats.failedInvoicesCount > 0 && <div>{stats.failedInvoicesCount} sikertelen</div>}
                                        {stats.overdueCount > 0 && <div>{stats.overdueCount} lejárt</div>}
                                    </div>
                                </div>
                            )}
                        </div>

                        {stats.createdInvoicesTotal > 0 && (
                            <div className="mb-5">
                                <div className="text-xs text-muted-foreground mb-1.5">Fizetettségi bontás</div>
                                <div className="h-3 w-full rounded-full overflow-hidden flex bg-muted">
                                    {stats.paidTotal > 0 && <div className="h-full bg-success" style={{ width: `${(stats.paidTotal / stats.createdInvoicesTotal) * 100}%` }} />}
                                    {stats.partiallyPaidTotal > 0 && <div className="h-full bg-warning" style={{ width: `${(stats.partiallyPaidTotal / stats.createdInvoicesTotal) * 100}%` }} />}
                                    {stats.expiredTotal > 0 && <div className="h-full bg-destructive" style={{ width: `${(stats.expiredTotal / stats.createdInvoicesTotal) * 100}%` }} />}
                                    {stats.outstandingTotal > 0 && <div className="h-full bg-muted-foreground/30" style={{ width: `${(stats.outstandingTotal / stats.createdInvoicesTotal) * 100}%` }} />}
                                </div>
                                <div className="flex flex-wrap gap-x-4 gap-y-1 mt-2 text-xs text-muted-foreground">
                                    <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-success" /> Kifizetve: {stats.paidTotal.toLocaleString('hu-HU')} HUF</span>
                                    {stats.partiallyPaidTotal > 0 && <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-warning" /> Részben kifizetve: {stats.partiallyPaidTotal.toLocaleString('hu-HU')} HUF</span>}
                                    {stats.expiredTotal > 0 && <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-destructive" /> Lejárt: {stats.expiredTotal.toLocaleString('hu-HU')} HUF</span>}
                                    <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-muted-foreground/30" /> Nyitva: {stats.outstandingTotal.toLocaleString('hu-HU')} HUF</span>
                                </div>
                            </div>
                        )}

                        {stats.revenueByClient.length > 1 && (
                            <div className="mb-5">
                                <div className="text-xs text-muted-foreground mb-1.5 flex items-center gap-1"><Users size={12} /> Bevétel ügyfelenként</div>
                                <div className="space-y-1.5">
                                    {stats.revenueByClient.map((c) => (
                                        <div key={c.name} className="flex items-center gap-2 text-xs">
                                            <span className="w-28 truncate text-muted-foreground flex-shrink-0">{c.name}</span>
                                            <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden">
                                                <div className="h-full bg-primary/70 rounded-full" style={{ width: `${(c.amount / stats.maxClientRevenue) * 100}%` }} />
                                            </div>
                                            <span className="w-28 text-right text-foreground flex-shrink-0">{c.amount.toLocaleString('hu-HU')} HUF</span>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}

                        {project && <RevenueAnalytics projectId={project.id} />}
                    </>
                )}
            </div>
        </div>
    );
}
