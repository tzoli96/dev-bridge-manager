// frontend/src/components/profitability/ScenariosTab.tsx
'use client';

import React from 'react';
import { profitabilityService, ParameterSet, Scenario } from '@/services/profitabilityService';
import ScenarioEditor from '@/components/profitability/ScenarioEditor';
import ParameterSetsModal from '@/components/profitability/ParameterSetsModal';
import { Button } from '@/components/ui/button';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { Plus } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

// null = a new, unsaved scenario; undefined = nothing selected.
type Selection = Scenario | null | undefined;

export default function ScenariosTab({ canManage }: { canManage: boolean }) {
    const [scenarios, setScenarios] = React.useState<Scenario[]>([]);
    const [parameterSets, setParameterSets] = React.useState<ParameterSet[]>([]);
    const [defaultCapacity, setDefaultCapacity] = React.useState(120);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [selected, setSelected] = React.useState<Selection>(undefined);
    const [showSets, setShowSets] = React.useState(false);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const [list, sets, settings] = await Promise.all([
                profitabilityService.listScenarios(),
                profitabilityService.listParameterSets(),
                profitabilityService.getSettings(),
            ]);
            setScenarios(list);
            setParameterSets(sets);
            setDefaultCapacity(settings.default_capacity_hours_per_month);
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        load();
    }, [load]);

    const reloadParameterSets = React.useCallback(async () => {
        try {
            setParameterSets(await profitabilityService.listParameterSets());
        } catch (err: unknown) {
            setError(errorMessage(err));
        }
    }, []);

    if (loading) return <LoadingState message="Forgatókönyvek betöltése..." />;
    if (error) return <ErrorState error={error} onRetry={load} />;

    return (
        <div className="grid gap-6 lg:grid-cols-[16rem_1fr]">
            <aside>
                <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-semibold text-foreground">Mentett</h2>
                    {canManage && (
                        <Button variant="secondary" size="sm" icon={Plus} onClick={() => setSelected(null)}>
                            Új
                        </Button>
                    )}
                </div>
                {scenarios.length === 0 && <p className="text-sm text-muted-foreground">Még nincs mentett forgatókönyv.</p>}
                <ul className="space-y-1">
                    {scenarios.map((s) => (
                        <li key={s.id}>
                            <button
                                onClick={() => setSelected(s)}
                                className={`w-full text-left px-3 py-2 rounded-md text-sm border ${
                                    selected && selected.id === s.id
                                        ? 'border-primary bg-primary/5 text-foreground'
                                        : 'border-border text-muted-foreground hover:text-foreground'
                                }`}
                            >
                                {s.name}
                                <span className="block text-xs text-muted-foreground">{s.horizon_months} hónap</span>
                            </button>
                        </li>
                    ))}
                </ul>
            </aside>

            <section>
                {selected === undefined ? (
                    <p className="text-sm text-muted-foreground">
                        Válassz egy mentett forgatókönyvet, vagy indíts újat. A számolás élőben frissül, mentés nélkül is.
                    </p>
                ) : (
                    <ScenarioEditor
                        key={selected ? selected.id : 'new'}
                        scenario={selected}
                        parameterSets={parameterSets}
                        defaultCapacity={defaultCapacity}
                        canManage={canManage}
                        onSaved={(saved) => {
                            setScenarios((prev) => (prev.some((p) => p.id === saved.id) ? prev.map((p) => (p.id === saved.id ? saved : p)) : [saved, ...prev]));
                            setSelected(saved);
                        }}
                        onDeleted={(id) => {
                            setScenarios((prev) => prev.filter((p) => p.id !== id));
                            setSelected(undefined);
                        }}
                        onOpenParameterSets={() => setShowSets(true)}
                    />
                )}
            </section>

            <ParameterSetsModal
                isOpen={showSets}
                onClose={() => setShowSets(false)}
                canManage={canManage}
                onChanged={reloadParameterSets}
            />
        </div>
    );
}
