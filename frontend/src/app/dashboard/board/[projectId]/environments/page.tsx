// frontend/src/app/dashboard/board/[projectId]/environments/page.tsx
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useProject } from '@/hooks/projects/use-project';
import {
    environmentsService,
    ProjectEnvironment,
    ProjectEnvironmentInput,
} from '@/services/environmentsService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Modal } from '@/components/ui/modal';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { ArrowLeft, Plus, Pencil, Trash2, Server, Globe, GitBranch, Copy } from 'lucide-react';

const emptyForm: ProjectEnvironmentInput = { name: '', url: '', git_repo_url: '' };

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

export default function ProjectEnvironmentsPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { project, loading: projectLoading, error: projectError } = useProject(projectId);

    const [environments, setEnvironments] = React.useState<ProjectEnvironment[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [actionError, setActionError] = React.useState<string | null>(null);

    const [isModalOpen, setIsModalOpen] = React.useState(false);
    const [editingId, setEditingId] = React.useState<number | null>(null);
    const [form, setForm] = React.useState<ProjectEnvironmentInput>(emptyForm);
    const [isSaving, setIsSaving] = React.useState(false);

    const loadEnvironments = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const data = await environmentsService.list(projectId);
            setEnvironments(data);
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, [projectId]);

    React.useEffect(() => {
        loadEnvironments();
    }, [loadEnvironments]);

    const openCreateModal = () => {
        setEditingId(null);
        setForm(emptyForm);
        setActionError(null);
        setIsModalOpen(true);
    };

    const openEditModal = (entry: ProjectEnvironment) => {
        setEditingId(entry.id);
        setForm({ name: entry.name, url: entry.url, git_repo_url: entry.git_repo_url });
        setActionError(null);
        setIsModalOpen(true);
    };

    const handleSave = async () => {
        if (!form.name.trim()) return;
        setIsSaving(true);
        try {
            if (editingId) {
                const updated = await environmentsService.update(projectId, editingId, form);
                setEnvironments((prev) => prev.map((e) => (e.id === editingId ? updated : e)));
            } else {
                const created = await environmentsService.create(projectId, form);
                setEnvironments((prev) => [...prev, created]);
            }
            setActionError(null);
            setIsModalOpen(false);
        } catch (err: unknown) {
            setActionError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const handleDelete = async (entry: ProjectEnvironment) => {
        if (!window.confirm(`Biztosan törlöd a(z) "${entry.name}" környezetet?`)) return;
        try {
            await environmentsService.remove(projectId, entry.id);
            setEnvironments((prev) => prev.filter((e) => e.id !== entry.id));
            setActionError(null);
        } catch (err: unknown) {
            setActionError(errorMessage(err));
        }
    };

    const copyToClipboard = async (value: string) => {
        await navigator.clipboard.writeText(value);
    };

    if (projectLoading) return <LoadingState message="Projekt betöltése..." />;
    if (projectError) return <ErrorState error={projectError} />;

    return (
        <div className="p-6 max-w-4xl mx-auto">
            <button
                onClick={() => router.push(`/dashboard/board/${projectId}`)}
                className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4"
            >
                <ArrowLeft size={14} /> Vissza a projekthez
            </button>

            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
                    <Server size={22} /> Környezetek{project ? ` — ${project.name}` : ''}
                </h1>
                <Button icon={Plus} onClick={openCreateModal}>
                    Új környezet
                </Button>
            </div>

            {actionError && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-4">
                    {actionError}
                </div>
            )}

            {loading && <LoadingState message="Környezetek betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={loadEnvironments} />}

            {!loading && !error && environments.length === 0 && (
                <EmptyState
                    icon="files"
                    title="Még nincs környezet ehhez a projekthez"
                    description="Adj hozzá környezeteket (pl. prod, dev, staging) URL-lel és git repóval."
                    action={{ label: 'Új környezet', onClick: openCreateModal }}
                />
            )}

            {!loading && !error && environments.length > 0 && (
                <div className="grid gap-4 sm:grid-cols-2">
                    {environments.map((entry) => (
                        <div
                            key={entry.id}
                            className="bg-card border border-border rounded-lg p-4 shadow-sm hover:shadow-md transition-shadow"
                        >
                            <div className="flex items-start justify-between gap-2">
                                <h3 className="font-medium text-foreground truncate">{entry.name}</h3>
                                <div className="flex items-center gap-1 flex-shrink-0">
                                    <Button variant="ghost" size="icon-sm" icon={Pencil} onClick={() => openEditModal(entry)} />
                                    <Button variant="ghost" size="icon-sm" icon={Trash2} onClick={() => handleDelete(entry)} />
                                </div>
                            </div>

                            {entry.url && (
                                <div className="mt-3 flex items-center justify-between gap-2 text-sm">
                                    <a
                                        href={entry.url}
                                        target="_blank"
                                        rel="noreferrer"
                                        className="inline-flex items-center gap-1.5 text-primary hover:underline break-all min-w-0"
                                    >
                                        <Globe size={14} className="flex-shrink-0" /> {entry.url}
                                    </a>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={Copy}
                                        onClick={() => copyToClipboard(entry.url)}
                                    />
                                </div>
                            )}

                            {entry.git_repo_url && (
                                <div className="mt-1 flex items-center justify-between gap-2 text-sm">
                                    <span className="inline-flex items-center gap-1.5 text-foreground break-all min-w-0">
                                        <GitBranch size={14} className="flex-shrink-0" /> {entry.git_repo_url}
                                    </span>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={Copy}
                                        onClick={() => copyToClipboard(entry.git_repo_url)}
                                    />
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}

            <Modal
                isOpen={isModalOpen}
                onClose={() => setIsModalOpen(false)}
                title={editingId ? 'Környezet szerkesztése' : 'Új környezet'}
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    {actionError && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {actionError}
                        </div>
                    )}
                    <Input
                        label="Név (pl. prod, dev)"
                        value={form.name}
                        onChange={(v) => setForm((f) => ({ ...f, name: v }))}
                    />
                    <Input
                        label="URL (https://...)"
                        value={form.url}
                        onChange={(v) => setForm((f) => ({ ...f, url: v }))}
                    />
                    <Input
                        label="Git repo (https://... vagy git@...)"
                        value={form.git_repo_url}
                        onChange={(v) => setForm((f) => ({ ...f, git_repo_url: v }))}
                    />
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setIsModalOpen(false)}>
                            Mégse
                        </Button>
                        <Button onClick={handleSave} loading={isSaving} disabled={!form.name.trim()}>
                            Mentés
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
}
