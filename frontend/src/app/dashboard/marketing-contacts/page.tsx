// frontend/src/app/dashboard/marketing-contacts/page.tsx
'use client';

import React from 'react';
import {
    marketingContactsService,
    MarketingContact,
    MarketingContactInput,
    MarketingContactFilters,
} from '@/services/marketingContactsService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Modal } from '@/components/ui/modal';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { Plus, Pencil, Trash2, Contact, Upload, Download } from 'lucide-react';
import type { ImportResult } from '@/services/marketingContactsService';

const emptyForm: MarketingContactInput = {
    email: '',
    first_name: '',
    last_name: '',
    subscribed: true,
    tags: '',
    source: '',
    notes: '',
};

const subscribedOptions = [
    { value: '', label: 'Mind' },
    { value: 'true', label: 'Feliratkozott' },
    { value: 'false', label: 'Leiratkozott' },
];

export default function MarketingContactsPage() {
    const [contacts, setContacts] = React.useState<MarketingContact[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [actionError, setActionError] = React.useState<string | null>(null);

    const [search, setSearch] = React.useState('');
    const [subscribedFilter, setSubscribedFilter] = React.useState('');
    const [tagFilter, setTagFilter] = React.useState('');

    const [isModalOpen, setIsModalOpen] = React.useState(false);
    const [editingId, setEditingId] = React.useState<number | null>(null);
    const [form, setForm] = React.useState<MarketingContactInput>(emptyForm);
    const [isSaving, setIsSaving] = React.useState(false);
    const [isImportModalOpen, setIsImportModalOpen] = React.useState(false);
    const [importFile, setImportFile] = React.useState<File | null>(null);
    const [importOverwrite, setImportOverwrite] = React.useState(false);
    const [isImporting, setIsImporting] = React.useState(false);
    const [importResult, setImportResult] = React.useState<ImportResult | null>(null);
    const [importError, setImportError] = React.useState<string | null>(null);
    const [isExporting, setIsExporting] = React.useState(false);

    const currentFilters = React.useCallback((): MarketingContactFilters => ({
        search: search.trim() || undefined,
        subscribed: subscribedFilter === '' ? undefined : subscribedFilter === 'true',
        tag: tagFilter.trim() || undefined,
    }), [search, subscribedFilter, tagFilter]);

    const requestIdRef = React.useRef(0);
    const hasLoadedRef = React.useRef(false);

    const loadContacts = React.useCallback(async () => {
        const requestId = ++requestIdRef.current;
        try {
            if (!hasLoadedRef.current) setLoading(true);
            setError(null);
            const data = await marketingContactsService.list(currentFilters());
            if (requestId !== requestIdRef.current) return;
            setContacts(data);
        } catch (err: any) {
            if (requestId !== requestIdRef.current) return;
            setError(err.message);
        } finally {
            if (requestId === requestIdRef.current) {
                setLoading(false);
                hasLoadedRef.current = true;
            }
        }
    }, [currentFilters]);

    React.useEffect(() => {
        const handle = setTimeout(() => {
            loadContacts();
        }, 300);
        return () => clearTimeout(handle);
    }, [loadContacts]);

    const openCreateModal = () => {
        setEditingId(null);
        setForm(emptyForm);
        setActionError(null);
        setIsModalOpen(true);
    };

    const openEditModal = (entry: MarketingContact) => {
        setEditingId(entry.id);
        setForm({
            email: entry.email,
            first_name: entry.first_name,
            last_name: entry.last_name,
            subscribed: entry.subscribed,
            tags: entry.tags,
            source: entry.source,
            notes: entry.notes,
        });
        setActionError(null);
        setIsModalOpen(true);
    };

    const handleSave = async () => {
        if (!form.email.trim()) return;
        setIsSaving(true);
        try {
            if (editingId) {
                const updated = await marketingContactsService.update(editingId, form);
                setContacts((prev) => prev.map((c) => (c.id === editingId ? updated : c)));
            } else {
                const created = await marketingContactsService.create(form);
                setContacts((prev) => [created, ...prev]);
            }
            setActionError(null);
            setIsModalOpen(false);
        } catch (err: any) {
            setActionError(err.message);
        } finally {
            setIsSaving(false);
        }
    };

    const handleDelete = async (entry: MarketingContact) => {
        if (!window.confirm(`Biztosan törlöd a(z) "${entry.email}" kontaktot?`)) return;
        try {
            await marketingContactsService.remove(entry.id);
            setContacts((prev) => prev.filter((c) => c.id !== entry.id));
            setActionError(null);
        } catch (err: any) {
            setActionError(err.message);
        }
    };

    const openImportModal = () => {
        setImportFile(null);
        setImportOverwrite(false);
        setImportResult(null);
        setImportError(null);
        setIsImportModalOpen(true);
    };

    const handleImport = async () => {
        if (!importFile) return;
        setIsImporting(true);
        setImportError(null);
        try {
            const result = await marketingContactsService.importCsv(importFile, importOverwrite);
            setImportResult(result);
            await loadContacts();
        } catch (err: any) {
            setImportError(err.message);
        } finally {
            setIsImporting(false);
        }
    };

    const handleExport = async () => {
        setIsExporting(true);
        try {
            const blob = await marketingContactsService.exportCsv(currentFilters());
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = 'marketing-contacts.csv';
            link.click();
            URL.revokeObjectURL(url);
        } catch (err: any) {
            setActionError(err.message);
        } finally {
            setIsExporting(false);
        }
    };

    return (
        <div className="p-6 max-w-6xl mx-auto space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
                    <Contact size={22} /> Marketing lista
                </h1>
                <div className="flex items-center gap-2">
                    <Button variant="secondary" icon={Upload} onClick={openImportModal}>
                        Importálás
                    </Button>
                    <Button variant="secondary" icon={Download} loading={isExporting} onClick={handleExport}>
                        Exportálás
                    </Button>
                    <Button icon={Plus} onClick={openCreateModal}>
                        Új kontakt
                    </Button>
                </div>
            </div>

            <div className="flex flex-wrap items-end gap-3">
                <div className="w-64">
                    <Input label="Keresés" value={search} onChange={setSearch} placeholder="email vagy név" />
                </div>
                <div className="w-48">
                    <Select
                        label="Állapot"
                        value={subscribedFilter}
                        onChange={setSubscribedFilter}
                        options={subscribedOptions}
                    />
                </div>
                <div className="w-48">
                    <Input label="Tag" value={tagFilter} onChange={setTagFilter} placeholder="pl. vip" />
                </div>
            </div>

            {actionError && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                    {actionError}
                </div>
            )}

            {loading && <LoadingState message="Kontaktok betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={loadContacts} />}

            {!loading && !error && contacts.length === 0 && (
                <EmptyState
                    icon="files"
                    title="Még nincs marketing kontakt"
                    description="Adj hozzá egy kontaktot, vagy importálj egy CSV listát."
                    action={{ label: 'Importálás', onClick: openImportModal }}
                />
            )}

            {!loading && !error && contacts.length > 0 && (
                <div className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-border">
                            <thead className="bg-muted">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Email</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Név</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Állapot</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Tag-ek</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Forrás</th>
                                    <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Műveletek</th>
                                </tr>
                            </thead>
                            <tbody className="bg-card divide-y divide-border">
                                {contacts.map((entry) => (
                                    <tr key={entry.id} className="hover:bg-muted/50">
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">{entry.email}</td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                            {[entry.first_name, entry.last_name].filter(Boolean).join(' ') || '—'}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span
                                                className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${
                                                    entry.subscribed
                                                        ? 'bg-success/10 text-success'
                                                        : 'bg-muted text-muted-foreground'
                                                }`}
                                            >
                                                {entry.subscribed ? 'Feliratkozott' : 'Leiratkozott'}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{entry.tags || '—'}</td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{entry.source || '—'}</td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right space-x-1">
                                            <Button variant="ghost" size="icon-sm" icon={Pencil} onClick={() => openEditModal(entry)} />
                                            <Button variant="ghost" size="icon-sm" icon={Trash2} onClick={() => handleDelete(entry)} />
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            <Modal
                isOpen={isModalOpen}
                onClose={() => setIsModalOpen(false)}
                title={editingId ? 'Kontakt szerkesztése' : 'Új kontakt'}
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    {actionError && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {actionError}
                        </div>
                    )}
                    <Input label="Email" value={form.email} onChange={(v) => setForm((f) => ({ ...f, email: v }))} />
                    <Input
                        label="Keresztnév"
                        value={form.first_name}
                        onChange={(v) => setForm((f) => ({ ...f, first_name: v }))}
                    />
                    <Input
                        label="Vezetéknév"
                        value={form.last_name}
                        onChange={(v) => setForm((f) => ({ ...f, last_name: v }))}
                    />
                    <label className="flex items-center gap-2 text-sm text-foreground">
                        <input
                            type="checkbox"
                            checked={form.subscribed}
                            onChange={(e) => setForm((f) => ({ ...f, subscribed: e.target.checked }))}
                        />
                        Feliratkozott
                    </label>
                    <Input
                        label="Tag-ek"
                        value={form.tags}
                        onChange={(v) => setForm((f) => ({ ...f, tags: v }))}
                        placeholder="pl. hírlevél, vip"
                    />
                    <Input label="Forrás" value={form.source} onChange={(v) => setForm((f) => ({ ...f, source: v }))} />
                    <Input label="Jegyzet" value={form.notes} onChange={(v) => setForm((f) => ({ ...f, notes: v }))} />
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setIsModalOpen(false)}>
                            Mégse
                        </Button>
                        <Button onClick={handleSave} loading={isSaving} disabled={!form.email.trim()}>
                            Mentés
                        </Button>
                    </div>
                </div>
            </Modal>

            <Modal
                isOpen={isImportModalOpen}
                onClose={() => setIsImportModalOpen(false)}
                title="Kontaktok importálása"
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    {importError && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {importError}
                        </div>
                    )}
                    {importResult && (
                        <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm space-y-1">
                            <p>
                                {importResult.created} új, {importResult.updated} felülírva, {importResult.skipped} kihagyva.
                            </p>
                            {importResult.errors.length > 0 && (
                                <ul className="list-disc list-inside text-xs text-muted-foreground max-h-32 overflow-y-auto">
                                    {importResult.errors.map((message, i) => (
                                        <li key={i}>{message}</li>
                                    ))}
                                </ul>
                            )}
                        </div>
                    )}
                    <input
                        type="file"
                        accept=".csv"
                        onChange={(e) => setImportFile(e.target.files?.[0] ?? null)}
                        className="block w-full text-sm text-foreground"
                    />
                    <label className="flex items-center gap-2 text-sm text-foreground">
                        <input
                            type="checkbox"
                            checked={importOverwrite}
                            onChange={(e) => setImportOverwrite(e.target.checked)}
                        />
                        Meglévők felülírása
                    </label>
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setIsImportModalOpen(false)}>
                            Bezárás
                        </Button>
                        <Button onClick={handleImport} loading={isImporting} disabled={!importFile}>
                            Importálás
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
}
