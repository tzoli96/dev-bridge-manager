// frontend/src/app/dashboard/board/[projectId]/page.tsx
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { boardService } from '@/services/kanban';
import { ProjectsService, Project } from '@/services/projectsService';
import { InvoicesService, Invoice } from '@/services/invoicesService';
import { useAuth } from '@/hooks/auth/use-auth';
import { hasPermission } from '@/utils/permissions';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, ArrowLeft, KanbanSquare, FileText } from 'lucide-react';
import type { Board } from '@/types/kanban';
import CreateInvoiceModal from '@/components/CreateInvoiceModal';

export default function BoardsListPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { user } = useAuth();
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [project, setProject] = React.useState<Project | null>(null);
    const [invoices, setInvoices] = React.useState<Invoice[]>([]);
    const [isLoading, setIsLoading] = React.useState(true);
    const [newBoardName, setNewBoardName] = React.useState('');
    const [isCreating, setIsCreating] = React.useState(false);
    const [isInvoiceModalOpen, setIsInvoiceModalOpen] = React.useState(false);

    const loadProjectAndInvoices = React.useCallback(() => {
        ProjectsService.getProject(Number(projectId)).then((p) => {
            setProject(p);
            if (hasPermission(user, 'invoices.read')) {
                InvoicesService.getProjectInvoices(p.id).then(setInvoices).catch(() => setInvoices([]));
            }
        }).catch(() => setProject(null));
    }, [projectId, user]);

    React.useEffect(() => {
        boardService.listBoards(projectId)
            .then((data) => setBoards(data))
            .finally(() => setIsLoading(false));

        loadProjectAndInvoices();
    }, [projectId, loadProjectAndInvoices]);

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

            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold text-foreground">Boards</h1>
                {project?.pricing_type && hasPermission(user, 'invoices.create') && (
                    <Button icon={FileText} onClick={() => setIsInvoiceModalOpen(true)}>
                        Számla kiállítása
                    </Button>
                )}
            </div>

            {project?.pricing_type && (
                <div className="mb-6 p-4 bg-card border border-border rounded-lg">
                    <h2 className="text-sm font-medium text-foreground mb-1">Pricing</h2>
                    <p className="text-sm text-muted-foreground">
                        {project.pricing_type === 'hourly'
                            ? `Hourly — ${project.hourly_rate} HUF/hour`
                            : `Fixed price — ${project.fixed_price} HUF`}
                    </p>
                </div>
            )}

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
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

            <div className="flex items-center gap-2 max-w-sm mb-6">
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

            {hasPermission(user, 'invoices.read') && (
                <div>
                    <h2 className="text-lg font-semibold text-foreground mb-3">Invoice history</h2>
                    {invoices.length === 0 ? (
                        <p className="text-sm text-muted-foreground">No invoices created yet.</p>
                    ) : (
                        <div className="space-y-2">
                            {invoices.map((invoice) => (
                                <div key={invoice.id} className="flex items-center justify-between p-3 bg-card border border-border rounded-lg text-sm">
                                    <div>
                                        <div className="font-medium text-foreground">{invoice.billingo_invoice_number || '—'}</div>
                                        <div className="text-muted-foreground">
                                            {invoice.client_name} · {invoice.amount} HUF
                                            {invoice.period_start && invoice.period_end && ` · ${invoice.period_start} - ${invoice.period_end}`}
                                        </div>
                                    </div>
                                    <span className={invoice.status === 'created' ? 'text-success' : 'text-destructive'}>
                                        {invoice.status}
                                    </span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            )}

            <CreateInvoiceModal
                isOpen={isInvoiceModalOpen}
                project={project}
                onClose={() => setIsInvoiceModalOpen(false)}
                onSuccess={() => {
                    setIsInvoiceModalOpen(false);
                    loadProjectAndInvoices();
                }}
            />
        </div>
    );
}
