'use client';

import React from 'react';
import {
    profitabilityService,
    ParameterSet,
    ParameterSetInput,
    PercentBase,
} from '@/services/profitabilityService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Modal } from '@/components/ui/modal';
import LoadingState from '@/components/ui/LoadingState';
import { formatHuf } from '@/utils/formatHuf';
import { Plus, Pencil, Trash2 } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

const BASE_OPTIONS = [
    { value: 'revenue', label: 'A bevétel %-a' },
    { value: 'after_costs', label: 'A költségek utáni eredmény %-a' },
];

interface PercentDraft {
    label: string;
    percent: string;
    base: PercentBase;
}

interface CostDraft {
    label: string;
    amount: string;
}

interface SetDraft {
    id: number | null;
    name: string;
    percentItems: PercentDraft[];
    costs: CostDraft[];
}

const emptyDraft: SetDraft = { id: null, name: '', percentItems: [], costs: [] };

// An empty or invalid number field counts as 0; the server validates the range.
const toNumber = (s: string) => {
    const n = Number(s.trim().replace(',', '.'));
    return Number.isFinite(n) ? n : 0;
};

const draftFromSet = (set: ParameterSet): SetDraft => ({
    id: set.id,
    name: set.name,
    percentItems: set.percent_items.map((p) => ({ label: p.label, percent: String(p.percent), base: p.base })),
    costs: set.fixed_monthly_costs.map((c) => ({ label: c.label, amount: String(c.amount) })),
});

const inputFromDraft = (d: SetDraft): ParameterSetInput => ({
    name: d.name,
    percent_items: d.percentItems.map((p) => ({ label: p.label, percent: toNumber(p.percent), base: p.base })),
    fixed_monthly_costs: d.costs.map((c) => ({ label: c.label, amount: toNumber(c.amount) })),
});

interface ParameterSetsModalProps {
    isOpen: boolean;
    onClose: () => void;
    canManage: boolean;
    onChanged: () => void;
}

export default function ParameterSetsModal({ isOpen, onClose, canManage, onChanged }: ParameterSetsModalProps) {
    const [sets, setSets] = React.useState<ParameterSet[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [draft, setDraft] = React.useState<SetDraft | null>(null);
    const [isSaving, setIsSaving] = React.useState(false);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            setSets(await profitabilityService.listParameterSets());
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        if (isOpen) {
            setDraft(null);
            load();
        }
    }, [isOpen, load]);

    const save = async () => {
        if (!draft) return;
        setIsSaving(true);
        try {
            const input = inputFromDraft(draft);
            if (draft.id === null) {
                await profitabilityService.createParameterSet(input);
            } else {
                await profitabilityService.updateParameterSet(draft.id, input);
            }
            setError(null);
            setDraft(null);
            await load();
            onChanged();
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const remove = async (set: ParameterSet) => {
        if (!window.confirm(`Biztosan törlöd a(z) "${set.name}" paraméterkészletet?`)) return;
        try {
            await profitabilityService.deleteParameterSet(set.id);
            setError(null);
            await load();
            onChanged();
        } catch (err: unknown) {
            setError(errorMessage(err));
        }
    };

    const patchPercent = (i: number, patch: Partial<PercentDraft>) =>
        setDraft((d) => (d ? { ...d, percentItems: d.percentItems.map((p, idx) => (idx === i ? { ...p, ...patch } : p)) } : d));
    const patchCost = (i: number, patch: Partial<CostDraft>) =>
        setDraft((d) => (d ? { ...d, costs: d.costs.map((c, idx) => (idx === i ? { ...c, ...patch } : c)) } : d));

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Paraméterkészletek" size="lg">
            <div className="space-y-4 mt-2">
                {error && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                        {error}
                    </div>
                )}

                {draft === null && (
                    <>
                        <p className="text-xs text-muted-foreground">
                            A készlet a forgatókönyv költségeit adja meg: fix havi költségeket és százalékos tételeket.
                            A rendszer nem tartalmaz adókulcsot vagy járulékot: minden értéket te adsz meg, és az eredmény
                            nem adótanács.
                        </p>

                        {loading && <LoadingState message="Készletek betöltése..." />}

                        {!loading && sets.length === 0 && (
                            <p className="text-sm text-muted-foreground">Még nincs paraméterkészlet.</p>
                        )}

                        {!loading &&
                            sets.map((set) => (
                                <div key={set.id} className="bg-card border border-border rounded-lg p-3">
                                    <div className="flex items-center justify-between gap-2">
                                        <h3 className="font-medium text-foreground">{set.name}</h3>
                                        {canManage && (
                                            <div className="flex items-center gap-1">
                                                <Button variant="ghost" size="icon-sm" icon={Pencil} aria-label={`${set.name} szerkesztése`} onClick={() => setDraft(draftFromSet(set))} />
                                                <Button variant="ghost" size="icon-sm" icon={Trash2} aria-label={`${set.name} törlése`} onClick={() => remove(set)} />
                                            </div>
                                        )}
                                    </div>
                                    <ul className="mt-2 text-xs text-muted-foreground space-y-0.5">
                                        {set.fixed_monthly_costs.map((c, i) => (
                                            <li key={`c${i}`}>
                                                {c.label}: {formatHuf(c.amount)} / hó
                                            </li>
                                        ))}
                                        {set.percent_items.map((p, i) => (
                                            <li key={`p${i}`}>
                                                {p.label}: {p.percent}% ({p.base === 'revenue' ? 'a bevételből' : 'a költségek utáni eredményből'})
                                            </li>
                                        ))}
                                        {set.fixed_monthly_costs.length === 0 && set.percent_items.length === 0 && <li>Nincs tétel.</li>}
                                    </ul>
                                </div>
                            ))}

                        {canManage && (
                            <div className="flex justify-end">
                                <Button icon={Plus} onClick={() => setDraft({ ...emptyDraft })}>
                                    Új készlet
                                </Button>
                            </div>
                        )}
                    </>
                )}

                {draft !== null && (
                    <div className="space-y-4">
                        <Input id="parameter-set-name" maxLength={255} label="Név" value={draft.name} onChange={(v) => setDraft((d) => (d ? { ...d, name: v } : d))} />

                        <section>
                            <h3 className="text-sm font-medium text-foreground mb-2">Fix havi költségek</h3>
                            <div className="space-y-2">
                                {draft.costs.map((c, i) => (
                                    <div key={i} className="flex items-end gap-2">
                                        <div className="flex-1">
                                            <Input id={`cost-${i}-label`} maxLength={100} label="Megnevezés" value={c.label} onChange={(v) => patchCost(i, { label: v })} />
                                        </div>
                                        <div className="w-40">
                                            <Input id={`cost-${i}-amount`} step="any" label="Összeg (Ft / hó)" type="number" value={c.amount} onChange={(v) => patchCost(i, { amount: v })} />
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            icon={Trash2}
                                            aria-label="Költség sor törlése"
                                            onClick={() => setDraft((d) => (d ? { ...d, costs: d.costs.filter((_, idx) => idx !== i) } : d))}
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="mt-2">
                                <Button
                                    variant="secondary"
                                    size="sm"
                                    icon={Plus}
                                    disabled={isSaving || draft.costs.length >= 20}
                                    onClick={() => setDraft((d) => (d ? { ...d, costs: [...d.costs, { label: '', amount: '0' }] } : d))}
                                >
                                    Költség
                                </Button>
                            </div>
                        </section>

                        <section>
                            <h3 className="text-sm font-medium text-foreground mb-2">Százalékos tételek</h3>
                            <div className="space-y-2">
                                {draft.percentItems.map((p, i) => (
                                    <div key={i} className="flex items-end gap-2">
                                        <div className="flex-1">
                                            <Input id={`pct-${i}-label`} maxLength={100} label="Megnevezés" value={p.label} onChange={(v) => patchPercent(i, { label: v })} />
                                        </div>
                                        <div className="w-24">
                                            <Input id={`pct-${i}-percent`} step="any" label="%" type="number" value={p.percent} onChange={(v) => patchPercent(i, { percent: v })} />
                                        </div>
                                        <div className="w-56">
                                            <Select
                                                id={`pct-${i}-base`}
                                                label="Alap"
                                                value={p.base}
                                                options={BASE_OPTIONS}
                                                onChange={(v) => patchPercent(i, { base: v as PercentBase })}
                                            />
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            icon={Trash2}
                                            aria-label="Százalékos tétel sor törlése"
                                            onClick={() => setDraft((d) => (d ? { ...d, percentItems: d.percentItems.filter((_, idx) => idx !== i) } : d))}
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="mt-2">
                                <Button
                                    variant="secondary"
                                    size="sm"
                                    icon={Plus}
                                    disabled={isSaving || draft.percentItems.length >= 20}
                                    onClick={() =>
                                        setDraft((d) =>
                                            d ? { ...d, percentItems: [...d.percentItems, { label: '', percent: '0', base: 'revenue' }] } : d
                                        )
                                    }
                                >
                                    Százalékos tétel
                                </Button>
                            </div>
                        </section>

                        <div className="flex justify-end gap-2 pt-2">
                            <Button
                                variant="secondary"
                                onClick={() => {
                                    setError(null);
                                    setDraft(null);
                                }}
                            >
                                Mégse
                            </Button>
                            <Button onClick={save} loading={isSaving} disabled={!draft.name.trim()}>
                                Mentés
                            </Button>
                        </div>
                    </div>
                )}
            </div>
        </Modal>
    );
}
