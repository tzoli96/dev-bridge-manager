// frontend/src/app/dashboard/profitability/page.tsx
'use client';

import React from 'react';
import { useAuth } from '@/hooks/auth/use-auth';
import { hasPermission } from '@/utils/permissions';
import {
    profitabilityService,
    ProfitabilityOverview,
    ProfitSettingsInput,
    RateRow,
} from '@/services/profitabilityService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Modal } from '@/components/ui/modal';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { TrendingUp, Settings2, Pencil } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

const formatHuf = (value: number | null) =>
    value === null ? '—' : `${Math.round(value).toLocaleString('hu-HU')} Ft`;
const formatHours = (value: number) => `${value.toFixed(1)} ó`;
const formatRatio = (value: number | null) => (value === null ? '—' : `${Math.round(value * 100)}%`);

export default function ProfitabilityPage() {
    const { user } = useAuth();
    const canManage = hasPermission(user, 'profitability.manage');

    const [months, setMonths] = React.useState(3);
    const [data, setData] = React.useState<ProfitabilityOverview | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [actionError, setActionError] = React.useState<string | null>(null);

    const [showSettings, setShowSettings] = React.useState(false);
    const [settingsForm, setSettingsForm] = React.useState<ProfitSettingsInput | null>(null);
    const [isSaving, setIsSaving] = React.useState(false);

    const [meetingClient, setMeetingClient] = React.useState<RateRow | null>(null);
    const [meetingHours, setMeetingHours] = React.useState('0');

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const overview = await profitabilityService.overview(months);
            setData(overview);
            setSettingsForm({
                minutes_per_inbound_email: overview.settings.minutes_per_inbound_email,
                minutes_per_outbound_email: overview.settings.minutes_per_outbound_email,
                default_capacity_hours_per_month: overview.settings.default_capacity_hours_per_month,
                underpriced_ratio_threshold: overview.settings.underpriced_ratio_threshold,
            });
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, [months]);

    React.useEffect(() => {
        load();
    }, [load]);

    const saveSettings = async () => {
        if (!settingsForm) return;
        setIsSaving(true);
        try {
            await profitabilityService.updateSettings(settingsForm);
            setActionError(null);
            setShowSettings(false);
            await load();
        } catch (err: unknown) {
            setActionError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const saveMeeting = async () => {
        if (!meetingClient) return;
        setIsSaving(true);
        try {
            await profitabilityService.setMeetingAllowance(meetingClient.id, Number(meetingHours));
            setActionError(null);
            setMeetingClient(null);
            await load();
        } catch (err: unknown) {
            setActionError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    // Cancelling must drop abandoned edits, so the form is re-seeded from the
    // last loaded settings before the modal closes.
    const closeSettings = () => {
        if (data) {
            setSettingsForm({
                minutes_per_inbound_email: data.settings.minutes_per_inbound_email,
                minutes_per_outbound_email: data.settings.minutes_per_outbound_email,
                default_capacity_hours_per_month: data.settings.default_capacity_hours_per_month,
                underpriced_ratio_threshold: data.settings.underpriced_ratio_threshold,
            });
        }
        setShowSettings(false);
    };

    const setField = (key: keyof ProfitSettingsInput) => (v: string) =>
        setSettingsForm((f) => (f ? { ...f, [key]: Number(v) } : f));

    const renderTable = (title: string, rows: RateRow[], clientTable: boolean) => (
        <section className="mb-8">
            <h2 className="text-lg font-semibold text-foreground mb-3">{title}</h2>
            <div className="overflow-x-auto bg-card border border-border rounded-lg">
                <table className="w-full text-sm">
                    <thead className="text-left text-muted-foreground border-b border-border">
                        <tr>
                            <th className="p-3">Név</th>
                            <th className="p-3 text-right">Bevétel</th>
                            <th className="p-3 text-right">Naplózott</th>
                            <th className="p-3 text-right">Levelezés</th>
                            {clientTable && <th className="p-3 text-right">Megbeszélés</th>}
                            <th className="p-3 text-right">Névleges óradíj</th>
                            <th className="p-3 text-right">Valódi óradíj</th>
                            <th className="p-3 text-right">Arány</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row) => (
                            <tr key={row.id} className="border-b border-border last:border-0">
                                <td className="p-3 text-foreground">
                                    {row.name}
                                    {row.underpriced_candidate && (
                                        <span className="ml-2 text-xs bg-destructive/10 text-destructive rounded-full px-2 py-0.5">
                                            áremelés-jelölt
                                        </span>
                                    )}
                                    {row.low_data && (
                                        <span
                                            className="ml-2 text-xs bg-muted text-muted-foreground rounded-full px-2 py-0.5"
                                            title={`Csak ${row.months_with_data} hónapnyi adat van, a szám tájékoztató jellegű`}
                                        >
                                            kevés adat
                                        </span>
                                    )}
                                </td>
                                <td className="p-3 text-right">{formatHuf(row.revenue)}</td>
                                <td className="p-3 text-right">{formatHours(row.logged_hours)}</td>
                                <td className="p-3 text-right">{formatHours(row.email_hours)}</td>
                                {clientTable && (
                                    <td className="p-3 text-right">
                                        {formatHours(row.meeting_hours)}
                                        {canManage && (
                                            <Button
                                                variant="ghost"
                                                size="icon-sm"
                                                icon={Pencil}
                                                onClick={() => {
                                                    setMeetingClient(row);
                                                    setMeetingHours(String(row.meeting_hours_per_month));
                                                    setActionError(null);
                                                }}
                                            />
                                        )}
                                    </td>
                                )}
                                <td className="p-3 text-right">{formatHuf(row.nominal_rate)}</td>
                                <td className="p-3 text-right font-medium">{formatHuf(row.real_rate)}</td>
                                <td className="p-3 text-right">{formatRatio(row.ratio)}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </section>
    );

    return (
        <div className="p-6 max-w-6xl mx-auto">
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
                    <TrendingUp size={22} /> Jövedelmezőség
                </h1>
                <div className="flex items-center gap-2">
                    <select
                        value={months}
                        onChange={(e) => setMonths(Number(e.target.value))}
                        className="border border-border rounded-md bg-background text-sm px-2 py-1.5"
                    >
                        {[3, 6, 12].map((m) => (
                            <option key={m} value={m}>
                                Utolsó {m} hónap
                            </option>
                        ))}
                    </select>
                    {canManage && (
                        <Button variant="secondary" icon={Settings2} onClick={() => {
                                setActionError(null);
                                setShowSettings(true);
                            }}>
                            Beállítások
                        </Button>
                    )}
                </div>
            </div>

            {actionError && !showSettings && meetingClient === null && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-4">
                    {actionError}
                </div>
            )}

            {loading && <LoadingState message="Jövedelmezőség betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={load} />}

            {!loading && !error && data && (
                <>
                    <p className="text-xs text-muted-foreground mb-4">
                        A valódi óradíj a bevételt a naplózott órákkal, a megbeszélés-átalánnyal és a levelezés becsült
                        idejével osztja. A levelezés ideje az ügyfélhez rendelt bejövő ügyfél-levelekből
                        ({data.settings.minutes_per_inbound_email} perc/levél) és az ugyanabban a szálban küldött
                        kimenő levelekből ({data.settings.minutes_per_outbound_email} perc/levél) becsült. Az
                        ügyfélhez nem rendelt szálak levelei nem számítanak bele, így az érték alsó becslés.
                        Az áremelés-jelölt küszöb: {Math.round(data.settings.underpriced_ratio_threshold * 100)}%.
                    </p>

                    {data.warnings.map((w) => (
                        <div
                            key={w}
                            className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm mb-4"
                        >
                            {w}
                        </div>
                    ))}

                    {data.clients.length === 0 && data.projects.length === 0 ? (
                        <EmptyState
                            icon="files"
                            title="Nincs adat az időszakban"
                            description="Nincs kiállított számla, naplózott óra vagy ügyfélhez rendelt levél az utolsó teljes hónapokban."
                        />
                    ) : (
                        <>
                            {renderTable('Ügyfelek', data.clients, true)}
                            {renderTable('Projektek', data.projects, false)}
                            <p className="text-xs text-muted-foreground">
                                A projekt-sorok nem tartalmazzák a megbeszélés-átalányt, mert az ügyfélenként adható meg.
                            </p>
                        </>
                    )}
                </>
            )}

            <Modal isOpen={showSettings} onClose={closeSettings} title="Számítási beállítások" size="sm">
                {settingsForm && (
                    <div className="space-y-3 mt-2">
                        {actionError && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {actionError}
                            </div>
                        )}
                        <Input
                            label="Perc / bejövő levél"
                            type="number"
                            value={settingsForm.minutes_per_inbound_email}
                            onChange={setField('minutes_per_inbound_email')}
                        />
                        <Input
                            label="Perc / kimenő levél"
                            type="number"
                            value={settingsForm.minutes_per_outbound_email}
                            onChange={setField('minutes_per_outbound_email')}
                        />
                        <Input
                            label="Kapacitás (óra / hó)"
                            type="number"
                            value={settingsForm.default_capacity_hours_per_month}
                            onChange={setField('default_capacity_hours_per_month')}
                        />
                        <Input
                            label="Áremelés-jelölt küszöb (0.01–1.5, pl. 0.6 = 60%)"
                            type="number"
                            value={settingsForm.underpriced_ratio_threshold}
                            onChange={setField('underpriced_ratio_threshold')}
                        />
                        <div className="flex justify-end gap-2 pt-2">
                            <Button variant="secondary" onClick={closeSettings}>
                                Mégse
                            </Button>
                            <Button onClick={saveSettings} loading={isSaving}>
                                Mentés
                            </Button>
                        </div>
                    </div>
                )}
            </Modal>

            <Modal
                isOpen={meetingClient !== null}
                onClose={() => setMeetingClient(null)}
                title={`Megbeszélés-átalány — ${meetingClient?.name ?? ''}`}
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    {actionError && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {actionError}
                        </div>
                    )}
                    <Input
                        label="Óra / hónap"
                        type="number"
                        value={meetingHours}
                        onChange={setMeetingHours}
                    />
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setMeetingClient(null)}>
                            Mégse
                        </Button>
                        <Button onClick={saveMeeting} loading={isSaving}>
                            Mentés
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
}
