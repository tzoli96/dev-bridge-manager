// frontend/src/app/dashboard/board/[projectId]/passwords/page.tsx
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useProject } from '@/hooks/projects/use-project';
import { passwordsService, ProjectPassword, ProjectPasswordInput } from '@/services/passwordsService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Modal } from '@/components/ui/modal';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { ArrowLeft, Plus, Eye, EyeOff, Copy, Pencil, Trash2, KeyRound } from 'lucide-react';

const emptyForm: ProjectPasswordInput = { title: '', username: '', password: '', url: '', notes: '' };

export default function ProjectPasswordsPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { project, loading: projectLoading, error: projectError } = useProject(projectId);

    const [passwords, setPasswords] = React.useState<ProjectPassword[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [actionError, setActionError] = React.useState<string | null>(null);
    const [revealed, setRevealed] = React.useState<Record<number, boolean>>({});

    const [isModalOpen, setIsModalOpen] = React.useState(false);
    const [editingId, setEditingId] = React.useState<number | null>(null);
    const [form, setForm] = React.useState<ProjectPasswordInput>(emptyForm);
    const [isSaving, setIsSaving] = React.useState(false);

    const loadPasswords = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const data = await passwordsService.list(projectId);
            setPasswords(data);
        } catch (err: any) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    }, [projectId]);

    React.useEffect(() => {
        loadPasswords();
    }, [loadPasswords]);

    const openCreateModal = () => {
        setEditingId(null);
        setForm(emptyForm);
        setActionError(null);
        setIsModalOpen(true);
    };

    const openEditModal = (entry: ProjectPassword) => {
        setEditingId(entry.id);
        setForm({
            title: entry.title,
            username: entry.username,
            password: entry.password,
            url: entry.url,
            notes: entry.notes,
        });
        setActionError(null);
        setIsModalOpen(true);
    };

    const handleSave = async () => {
        if (!form.title.trim() || !form.password.trim()) return;
        setIsSaving(true);
        try {
            if (editingId) {
                const updated = await passwordsService.update(projectId, editingId, form);
                setPasswords((prev) => prev.map((p) => (p.id === editingId ? updated : p)));
            } else {
                const created = await passwordsService.create(projectId, form);
                setPasswords((prev) => [created, ...prev]);
            }
            setActionError(null);
            setIsModalOpen(false);
        } catch (err: any) {
            setActionError(err.message);
        } finally {
            setIsSaving(false);
        }
    };

    const handleDelete = async (entry: ProjectPassword) => {
        if (!window.confirm(`Biztosan törlöd a(z) "${entry.title}" jelszót?`)) return;
        try {
            await passwordsService.remove(projectId, entry.id);
            setPasswords((prev) => prev.filter((p) => p.id !== entry.id));
            setActionError(null);
        } catch (err: any) {
            setActionError(err.message);
        }
    };

    const toggleReveal = (id: number) => {
        setRevealed((prev) => ({ ...prev, [id]: !prev[id] }));
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
                    <KeyRound size={22} /> Jelszavak{project ? ` — ${project.name}` : ''}
                </h1>
                <Button icon={Plus} onClick={openCreateModal}>
                    Új jelszó
                </Button>
            </div>

            {actionError && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-4">
                    {actionError}
                </div>
            )}

            {loading && <LoadingState message="Jelszavak betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={loadPasswords} />}

            {!loading && !error && passwords.length === 0 && (
                <EmptyState
                    icon="files"
                    title="Még nincs jelszó ehhez a projekthez"
                    description="Adj hozzá egy megosztott hitelesítő adatot (szerver, adatbázis, API kulcs), amit a projekt tagjai láthatnak."
                    action={{ label: 'Új jelszó', onClick: openCreateModal }}
                />
            )}

            {!loading && !error && passwords.length > 0 && (
                <div className="grid gap-4 sm:grid-cols-2">
                    {passwords.map((entry) => (
                        <div
                            key={entry.id}
                            className="bg-card border border-border rounded-lg p-4 shadow-sm hover:shadow-md transition-shadow"
                        >
                            <div className="flex items-start justify-between gap-2">
                                <div className="min-w-0">
                                    <h3 className="font-medium text-foreground truncate">{entry.title}</h3>
                                    {entry.url && (
                                        <a
                                            href={entry.url}
                                            target="_blank"
                                            rel="noreferrer"
                                            className="text-xs text-primary hover:underline break-all"
                                        >
                                            {entry.url}
                                        </a>
                                    )}
                                </div>
                                <div className="flex items-center gap-1 flex-shrink-0">
                                    <Button variant="ghost" size="icon-sm" icon={Pencil} onClick={() => openEditModal(entry)} />
                                    <Button variant="ghost" size="icon-sm" icon={Trash2} onClick={() => handleDelete(entry)} />
                                </div>
                            </div>

                            {entry.username && (
                                <div className="mt-3 flex items-center justify-between text-sm">
                                    <span className="text-muted-foreground truncate">{entry.username}</span>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={Copy}
                                        onClick={() => copyToClipboard(entry.username)}
                                    />
                                </div>
                            )}

                            <div className="mt-1 flex items-center justify-between text-sm">
                                <span className="font-mono text-foreground">
                                    {revealed[entry.id] ? entry.password : '••••••••'}
                                </span>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={revealed[entry.id] ? EyeOff : Eye}
                                        onClick={() => toggleReveal(entry.id)}
                                    />
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={Copy}
                                        onClick={() => copyToClipboard(entry.password)}
                                    />
                                </div>
                            </div>

                            {entry.notes && <p className="mt-3 text-xs text-muted-foreground">{entry.notes}</p>}
                        </div>
                    ))}
                </div>
            )}

            <Modal
                isOpen={isModalOpen}
                onClose={() => setIsModalOpen(false)}
                title={editingId ? 'Jelszó szerkesztése' : 'Új jelszó'}
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    <Input label="Cím" value={form.title} onChange={(v) => setForm((f) => ({ ...f, title: v }))} />
                    <Input
                        label="Felhasználónév"
                        value={form.username}
                        onChange={(v) => setForm((f) => ({ ...f, username: v }))}
                    />
                    <Input
                        label="Jelszó"
                        type="password"
                        value={form.password}
                        onChange={(v) => setForm((f) => ({ ...f, password: v }))}
                    />
                    <Input label="URL" value={form.url} onChange={(v) => setForm((f) => ({ ...f, url: v }))} />
                    <Input label="Jegyzet" value={form.notes} onChange={(v) => setForm((f) => ({ ...f, notes: v }))} />
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setIsModalOpen(false)}>
                            Mégse
                        </Button>
                        <Button
                            onClick={handleSave}
                            loading={isSaving}
                            disabled={!form.title.trim() || !form.password.trim()}
                        >
                            Mentés
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
}
