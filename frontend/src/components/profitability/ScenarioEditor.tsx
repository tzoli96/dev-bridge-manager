// frontend/src/components/profitability/ScenarioEditor.tsx
'use client';

import React from 'react';
import {
    profitabilityService,
    ParameterSet,
    Scenario,
    ScenarioAdjustment,
    ScenarioInput,
    ScenarioNewClient,
    ScenarioResult,
} from '@/services/profitabilityService';
import ScenarioResultPanel from '@/components/profitability/ScenarioResultPanel';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { formatHuf } from '@/utils/formatHuf';
import { Plus, Trash2, Settings2 } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

interface AdjustmentDraft {
    rate: string;
    fixed: string;
    delta: string;
    drops: boolean;
}

interface NewClientDraft {
    key: number;
    name: string;
    revenue: string;
    hours: string;
}

interface Draft {
    name: string;
    horizon: string;
    capacity: string;
    parameterSetId: string;
    adjustments: Record<number, AdjustmentDraft>;
    newClients: NewClientDraft[];
}

// Server limit for new clients per scenario.
const MAX_NEW_CLIENTS = 50;

const emptyAdjustment: AdjustmentDraft = { rate: '', fixed: '', delta: '', drops: false };

// null = empty or not a number.
const parseNum = (s: string): number | null => {
    const t = s.trim().replace(',', '.');
    if (t === '') return null;
    const n = Number(t);
    return Number.isFinite(n) ? n : null;
};

// Builds the request body, or null while the draft cannot be priced yet
// (no parameter set / capacity). Half-filled rows are left out so that live
// typing does not trigger validation errors.
function toInput(draft: Draft): ScenarioInput | null {
    const capacity = parseNum(draft.capacity);
    const setId = Number(draft.parameterSetId);
    const horizon = Number(draft.horizon);
    if (capacity === null || capacity <= 0 || !Number.isFinite(setId) || setId <= 0) return null;

    const adjustments: ScenarioAdjustment[] = [];
    for (const [clientId, a] of Object.entries(draft.adjustments)) {
        const rate = parseNum(a.rate);
        const fixed = parseNum(a.fixed);
        const delta = parseNum(a.delta) ?? 0;
        if (!a.drops && rate === null && fixed === null && delta === 0) continue;
        adjustments.push({
            client_id: Number(clientId),
            new_hourly_rate: rate,
            new_fixed_price: fixed,
            hours_delta: delta,
            drops: a.drops,
        });
    }

    const newClients: ScenarioNewClient[] = draft.newClients
        .filter((n) => n.name.trim() !== '')
        .map((n) => ({
            name: n.name.trim(),
            monthly_revenue: parseNum(n.revenue) ?? 0,
            monthly_hours: parseNum(n.hours) ?? 0,
        }));

    return {
        name: draft.name.trim(),
        horizon_months: horizon,
        parameter_set_id: setId,
        capacity_hours_per_month: capacity,
        client_adjustments: adjustments,
        new_clients: newClients,
    };
}

function draftFromScenario(s: Scenario): Draft {
    const adjustments: Record<number, AdjustmentDraft> = {};
    for (const a of s.client_adjustments) {
        adjustments[a.client_id] = {
            rate: a.new_hourly_rate === null ? '' : String(a.new_hourly_rate),
            fixed: a.new_fixed_price === null ? '' : String(a.new_fixed_price),
            delta: a.hours_delta === 0 ? '' : String(a.hours_delta),
            drops: a.drops,
        };
    }
    return {
        name: s.name,
        horizon: String(s.horizon_months),
        capacity: String(s.capacity_hours_per_month),
        parameterSetId: String(s.parameter_set_id),
        adjustments,
        newClients: s.new_clients.map((n, i) => ({
            key: i,
            name: n.name,
            revenue: String(n.monthly_revenue),
            hours: String(n.monthly_hours),
        })),
    };
}

interface ScenarioEditorProps {
    scenario: Scenario | null;
    parameterSets: ParameterSet[];
    defaultCapacity: number;
    canManage: boolean;
    onSaved: (scenario: Scenario) => void;
    onDeleted: (id: number) => void;
    onOpenParameterSets: () => void;
}

export default function ScenarioEditor({
    scenario,
    parameterSets,
    defaultCapacity,
    canManage,
    onSaved,
    onDeleted,
    onOpenParameterSets,
}: ScenarioEditorProps) {
    const [draft, setDraft] = React.useState<Draft>(() =>
        scenario
            ? draftFromScenario(scenario)
            : {
                  name: '',
                  horizon: '6',
                  capacity: String(defaultCapacity),
                  parameterSetId: parameterSets[0] ? String(parameterSets[0].id) : '',
                  adjustments: {},
                  newClients: [],
              }
    );
    const [result, setResult] = React.useState<ScenarioResult | null>(null);
    const [computing, setComputing] = React.useState(false);
    const [saveError, setSaveError] = React.useState<string | null>(null);
    const [computeError, setComputeError] = React.useState<string | null>(null);
    const [isSaving, setIsSaving] = React.useState(false);
    const requestId = React.useRef(0);
    const nextKey = React.useRef(1000);

    // If the parameter sets load after the editor opened (or the chosen one was
    // deleted), fall back to the first available one.
    React.useEffect(() => {
        if (parameterSets.length === 0) return;
        const exists = parameterSets.some((p) => String(p.id) === draft.parameterSetId);
        if (!exists && !scenario) {
            setDraft((d) => ({ ...d, parameterSetId: String(parameterSets[0].id) }));
        }
    }, [parameterSets, draft.parameterSetId, scenario]);

    // Live pricing: wait 400 ms after the last change and drop stale answers.
    React.useEffect(() => {
        const input = toInput(draft);
        if (!input) {
            setComputing(false);
            setResult(null);
            setComputeError(null);
            return;
        }
        const id = ++requestId.current;
        const timer = setTimeout(async () => {
            try {
                setComputing(true);
                const r = await profitabilityService.computeScenario(input);
                if (id === requestId.current) {
                    setResult(r);
                    setComputeError(null);
                }
            } catch (err: unknown) {
                if (id === requestId.current) setComputeError(errorMessage(err));
            } finally {
                if (id === requestId.current) setComputing(false);
            }
        }, 400);
        // Invalidates any in-flight request when the draft changes or the editor unmounts.
        const invalidate = () => {
            requestId.current++;
        };
        return () => {
            clearTimeout(timer);
            invalidate();
        };
        // parameterSets is only a trigger: editing sets must reprice the draft.
    }, [draft, parameterSets]);

    const patchAdjustment = (clientId: number, patch: Partial<AdjustmentDraft>) =>
        setDraft((d) => ({
            ...d,
            adjustments: { ...d.adjustments, [clientId]: { ...(d.adjustments[clientId] ?? emptyAdjustment), ...patch } },
        }));

    const patchNewClient = (key: number, patch: Partial<NewClientDraft>) =>
        setDraft((d) => ({ ...d, newClients: d.newClients.map((n) => (n.key === key ? { ...n, ...patch } : n)) }));

    const save = async () => {
        const input = toInput(draft);
        if (!input || !input.name) {
            setSaveError('A mentéshez adj nevet, válassz paraméterkészletet és adj meg kapacitást.');
            return;
        }
        setIsSaving(true);
        try {
            const saved = scenario
                ? await profitabilityService.updateScenario(scenario.id, input)
                : await profitabilityService.createScenario(input);
            setSaveError(null);
            onSaved(saved);
        } catch (err: unknown) {
            setSaveError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const remove = async () => {
        if (!scenario) return;
        if (!window.confirm(`Biztosan törlöd a(z) "${scenario.name}" forgatókönyvet?`)) return;
        try {
            await profitabilityService.deleteScenario(scenario.id);
            onDeleted(scenario.id);
        } catch (err: unknown) {
            setSaveError(errorMessage(err));
        }
    };

    const clientRows = result ? result.clients.filter((c) => !c.is_new) : [];

    return (
        <div className="space-y-5">
            {saveError && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">{saveError}</div>
            )}

            <div className="grid gap-3 sm:grid-cols-2">
                <Input
                    id="scenario-name"
                    label="Név"
                    maxLength={255}
                    value={draft.name}
                    onChange={(v) => setDraft((d) => ({ ...d, name: v }))}
                />
                <Select
                    id="scenario-horizon"
                    label="Időtáv"
                    value={draft.horizon}
                    options={[3, 4, 5, 6].map((m) => ({ value: String(m), label: `${m} hónap` }))}
                    onChange={(v) => setDraft((d) => ({ ...d, horizon: v }))}
                />
                <Input
                    id="scenario-capacity"
                    label="Kapacitás (óra / hó)"
                    type="number"
                    step="any"
                    value={draft.capacity}
                    onChange={(v) => setDraft((d) => ({ ...d, capacity: v }))}
                />
                <div className="flex items-end gap-2">
                    <div className="flex-1">
                        <Select
                            id="scenario-parameter-set"
                            label="Paraméterkészlet"
                            value={draft.parameterSetId}
                            options={parameterSets.map((p) => ({ value: String(p.id), label: p.name }))}
                            placeholder="Válassz készletet"
                            onChange={(v) => setDraft((d) => ({ ...d, parameterSetId: v }))}
                        />
                    </div>
                    <Button variant="secondary" icon={Settings2} onClick={onOpenParameterSets}>
                        Készletek
                    </Button>
                </div>
            </div>

            {parameterSets.length === 0 && (
                <p className="text-sm text-muted-foreground">
                    Az eredményhez előbb hozz létre egy paraméterkészletet (az akár üres is lehet).
                </p>
            )}

            <section>
                <h3 className="text-sm font-medium text-foreground mb-2">Ügyfelek módosítása</h3>
                {!result && <p className="text-sm text-muted-foreground">Az ügyfelek az első számolás után jelennek meg.</p>}
                {result && clientRows.length === 0 && (
                    <p className="text-sm text-muted-foreground">Nincs rendszeres bevételű ügyfél az alapidőszakban.</p>
                )}
                {clientRows.length > 0 && (
                    <div className="overflow-x-auto bg-card border border-border rounded-lg">
                        <table className="w-full text-sm">
                            <thead className="text-left text-muted-foreground border-b border-border">
                                <tr>
                                    <th className="p-3">Ügyfél</th>
                                    <th className="p-3 text-right">Havi bevétel</th>
                                    <th className="p-3 text-right">Havi óra</th>
                                    <th className="p-3">Új óradíj (Ft)</th>
                                    <th className="p-3">Új fix ár (Ft)</th>
                                    <th className="p-3">Óraváltozás</th>
                                    <th className="p-3">Kiesik</th>
                                    <th className="p-3 text-right">Új bevétel</th>
                                </tr>
                            </thead>
                            <tbody>
                                {clientRows.map((c) => {
                                    const a = draft.adjustments[c.client_id] ?? emptyAdjustment;
                                    return (
                                        <tr key={c.client_id} className="border-b border-border last:border-0 align-middle">
                                            <td className="p-3 text-foreground">{c.name}</td>
                                            <td className="p-3 text-right">{formatHuf(c.base_revenue)}</td>
                                            <td className="p-3 text-right">{c.base_hours.toFixed(1).replace('.', ',')} ó</td>
                                            <td className="p-2 w-32">
                                                <Input
                                                    id={`adj-${c.client_id}-rate`}
                                                    aria-label={`${c.name} új óradíj`}
                                                    type="number"
                                                    step="any"
                                                    value={a.rate}
                                                    onChange={(v) => patchAdjustment(c.client_id, { rate: v, ...(v.trim() !== '' ? { fixed: '' } : {}) })}
                                                />
                                            </td>
                                            <td className="p-2 w-32">
                                                <Input
                                                    id={`adj-${c.client_id}-fixed`}
                                                    aria-label={`${c.name} új fix ár`}
                                                    type="number"
                                                    step="any"
                                                    value={a.fixed}
                                                    onChange={(v) => patchAdjustment(c.client_id, { fixed: v, ...(v.trim() !== '' ? { rate: '' } : {}) })}
                                                />
                                            </td>
                                            <td className="p-2 w-24">
                                                <Input
                                                    id={`adj-${c.client_id}-delta`}
                                                    aria-label={`${c.name} óraváltozás`}
                                                    type="number"
                                                    step="any"
                                                    value={a.delta}
                                                    onChange={(v) => patchAdjustment(c.client_id, { delta: v })}
                                                />
                                            </td>
                                            <td className="p-3">
                                                <input
                                                    type="checkbox"
                                                    checked={a.drops}
                                                    aria-label={`${c.name} kiesik`}
                                                    onChange={(e) => patchAdjustment(c.client_id, { drops: e.target.checked })}
                                                />
                                            </td>
                                            <td className="p-3 text-right font-medium">{c.dropped ? '—' : formatHuf(c.revenue)}</td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                )}
                <p className="text-xs text-muted-foreground mt-2">
                    Új óradíj és új fix ár egyszerre nem adható meg. Az óraváltozás a naplózott órákra vonatkozik; ár nélkül a bevétel a
                    jelenlegi óradíjon arányosan változik.
                </p>
            </section>

            <section>
                <h3 className="text-sm font-medium text-foreground mb-2">Új ügyfelek</h3>
                <div className="space-y-2">
                    {draft.newClients.map((n) => (
                        <div key={n.key} className="flex items-end gap-2">
                            <div className="flex-1">
                                <Input
                                    id={`new-${n.key}-name`}
                                    label="Név"
                                    maxLength={255}
                                    value={n.name}
                                    onChange={(v) => patchNewClient(n.key, { name: v })}
                                />
                            </div>
                            <div className="w-40">
                                <Input
                                    id={`new-${n.key}-revenue`}
                                    label="Havi bevétel (Ft)"
                                    type="number"
                                    step="any"
                                    value={n.revenue}
                                    onChange={(v) => patchNewClient(n.key, { revenue: v })}
                                />
                            </div>
                            <div className="w-32">
                                <Input
                                    id={`new-${n.key}-hours`}
                                    label="Havi óra"
                                    type="number"
                                    step="any"
                                    value={n.hours}
                                    onChange={(v) => patchNewClient(n.key, { hours: v })}
                                />
                            </div>
                            <Button
                                variant="ghost"
                                size="icon-sm"
                                icon={Trash2}
                                aria-label="Új ügyfél törlése"
                                onClick={() => setDraft((d) => ({ ...d, newClients: d.newClients.filter((x) => x.key !== n.key) }))}
                            />
                        </div>
                    ))}
                </div>
                <div className="mt-2">
                    <Button
                        variant="secondary"
                        icon={Plus}
                        disabled={draft.newClients.length >= MAX_NEW_CLIENTS}
                        onClick={() =>
                            setDraft((d) => ({
                                ...d,
                                newClients: [...d.newClients, { key: nextKey.current++, name: '', revenue: '0', hours: '0' }],
                            }))
                        }
                    >
                        Új ügyfél
                    </Button>
                </div>
            </section>

            <section>
                <h3 className="text-sm font-medium text-foreground mb-2">
                    Eredmény {computing && <span className="text-xs font-normal text-muted-foreground">(számolás...)</span>}
                </h3>
                {result && computeError && (
                    <p className="text-sm text-muted-foreground mb-2">
                        A megjelenített eredmény nem friss, mert a legutóbbi számítás hibát adott: {computeError}
                    </p>
                )}
                {result ? (
                    <div className={computeError ? 'opacity-60' : undefined}>
                        <ScenarioResultPanel result={result} />
                    </div>
                ) : computeError ? (
                    <p className="text-sm text-destructive">{computeError}</p>
                ) : (
                    <p className="text-sm text-muted-foreground">Válassz paraméterkészletet és adj meg kapacitást az eredményhez.</p>
                )}
            </section>

            {canManage && (
                <div className="flex justify-end gap-2">
                    {scenario && (
                        <Button variant="secondary" icon={Trash2} onClick={remove}>
                            Törlés
                        </Button>
                    )}
                    <Button onClick={save} loading={isSaving} disabled={!draft.name.trim()}>
                        Mentés
                    </Button>
                </div>
            )}
        </div>
    );
}
