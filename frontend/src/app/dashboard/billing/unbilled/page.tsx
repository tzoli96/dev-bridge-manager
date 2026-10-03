'use client';

import React from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/auth/use-auth';
import { hasPermission } from '@/utils/permissions';
import { UnbilledHoursService, UnbilledHours } from '@/services/unbilledHoursService';
import { Button } from '@/components/ui/button';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { formatHuf } from '@/utils/formatHuf';
import { ArrowLeft, Receipt } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

const formatHours = (v: number) => `${v.toFixed(1).replace('.', ',')} ó`;

const pricingLabel: Record<string, string> = { fixed: 'Fix áras', hobby: 'Hobbi' };

export default function UnbilledHoursPage() {
    const router = useRouter();
    const { user } = useAuth();
    const canInvoice = hasPermission(user, 'invoices.create');

    const [data, setData] = React.useState<UnbilledHours | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            setData(await UnbilledHoursService.get());
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        load();
    }, [load]);

    const partial = data ? data.clients.some((g) => g.projects.some((p) => p.missing_rate)) : false;

    return (
        <div className="p-6 max-w-6xl space-y-6">
            <div>
                <button
                    onClick={() => router.push('/dashboard/billing')}
                    className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-3"
                >
                    <ArrowLeft size={14} /> Vissza a számlázáshoz
                </button>
                <h1 className="text-2xl font-bold text-foreground mb-1">Számlázatlan órák</h1>
                <p className="text-sm text-muted-foreground">
                    Számlázandó: a kész feladatok és az előző hónapok még ki nem számlázott órái. Folyamatban: az aktuális hónap órái
                    nyitott feladatokon.
                </p>
                <p className="text-xs text-muted-foreground mt-1">
                    A számla időszakra szól: a kiválasztott napok összes órája számlázódik, a folyamatban lévők is.
                </p>
            </div>

            {loading && <LoadingState message="Számlázatlan órák betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={load} />}

            {!loading && !error && data && (
                <>
                    <div className="grid gap-4 sm:grid-cols-3">
                        <div className="bg-card border border-border rounded-lg p-4">
                            <p className="text-xs text-muted-foreground">Számlázandó óra</p>
                            <p className="text-2xl font-semibold text-foreground">{formatHours(data.totals.billable_hours)}</p>
                        </div>
                        <div className="bg-card border border-border rounded-lg p-4">
                            <p className="text-xs text-muted-foreground">Becsült összeg (nettó)</p>
                            <p className="text-2xl font-semibold text-foreground">{formatHuf(data.totals.billable_amount)}</p>
                            {partial && <p className="text-xs text-muted-foreground">Óradíj nélküli projekt(ek) nélkül</p>}
                        </div>
                        <div className="bg-card border border-border rounded-lg p-4">
                            <p className="text-xs text-muted-foreground">Folyamatban</p>
                            <p className="text-2xl font-semibold text-foreground">{formatHours(data.totals.in_progress_hours)}</p>
                        </div>
                    </div>

                    {data.warnings.map((w, i) => (
                        <div key={`${i}-${w}`} className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm">
                            {w}
                        </div>
                    ))}

                    {data.clients.length === 0 ? (
                        <EmptyState
                            icon="files"
                            title="Nincs számlázandó óra"
                            description="Minden rögzített óra ki van számlázva, vagy még nincs óra rögzítve."
                        />
                    ) : (
                        data.clients.map((group) => (
                            <section key={group.client_id ?? 'unassigned'}>
                                <div className="flex items-baseline justify-between mb-2">
                                    <h2 className="text-lg font-semibold text-foreground">{group.client_name}</h2>
                                    <p className="text-sm text-muted-foreground">
                                        Számlázandó: {formatHours(group.billable_hours)} · {formatHuf(group.billable_amount)}
                                        {group.projects.some((p) => p.missing_rate) && ' (óradíj nélküli projekt nélkül)'}
                                    </p>
                                </div>
                                <div className="overflow-x-auto bg-card border border-border rounded-lg">
                                    <table className="w-full text-sm">
                                        <thead className="text-left text-muted-foreground border-b border-border">
                                            <tr>
                                                <th scope="col" className="p-3">Projekt</th>
                                                <th scope="col" className="p-3 text-right">Számlázandó</th>
                                                <th scope="col" className="p-3 text-right">Becsült összeg</th>
                                                <th scope="col" className="p-3 text-right">Folyamatban</th>
                                                <th scope="col" className="p-3 text-right">Legrégebbi</th>
                                                <th scope="col" className="p-3"></th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {group.projects.map((p) => (
                                                <tr key={p.project_id} className="border-b border-border last:border-0 align-middle">
                                                    <td className="p-3 text-foreground">
                                                        {p.project_name}
                                                        {!p.hourly_based && (
                                                            <span className="ml-2 text-xs bg-muted text-muted-foreground rounded-full px-2 py-0.5">
                                                                {pricingLabel[p.pricing_type] ?? p.pricing_type} · nem óra alapú
                                                            </span>
                                                        )}
                                                    </td>
                                                    {p.hourly_based ? (
                                                        <>
                                                            <td className="p-3 text-right">{formatHours(p.billable_hours)}</td>
                                                            <td className="p-3 text-right">
                                                                {p.billable_amount !== null ? (
                                                                    formatHuf(p.billable_amount)
                                                                ) : p.missing_rate ? (
                                                                    <span className="text-warning" title="A projektnek nincs óradíja">
                                                                        nincs óradíj
                                                                    </span>
                                                                ) : (
                                                                    '—'
                                                                )}
                                                            </td>
                                                            <td className="p-3 text-right text-muted-foreground">
                                                                {formatHours(p.in_progress_hours)}
                                                            </td>
                                                            <td className="p-3 text-right text-muted-foreground">
                                                                {p.oldest_billable_date ?? '—'}
                                                            </td>
                                                        </>
                                                    ) : (
                                                        <>
                                                            <td className="p-3 text-right text-muted-foreground">—</td>
                                                            <td className="p-3 text-right text-muted-foreground">—</td>
                                                            <td className="p-3 text-right text-muted-foreground">—</td>
                                                            <td className="p-3 text-right text-muted-foreground" title="Rögzített órák összesen">
                                                                {formatHours(p.logged_hours)} rögzítve
                                                            </td>
                                                        </>
                                                    )}
                                                    <td className="p-3 text-right">
                                                        {canInvoice && p.hourly_based && p.billable_hours > 0 && (
                                                            <Button
                                                                variant="secondary"
                                                                size="sm"
                                                                icon={Receipt}
                                                                onClick={() => router.push(`/dashboard/board/${p.project_id}/invoice`)}
                                                            >
                                                                Számlázás
                                                            </Button>
                                                        )}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </section>
                        ))
                    )}

                    <p className="text-xs text-muted-foreground">
                        {data.fully_invoiced_projects > 0 &&
                            `${data.fully_invoiced_projects} óradíjas projekt teljesen ki van számlázva, ezek nem szerepelnek a listában. `}
                        A „kiszámlázott” állapot a kiállított óradíjas számlák időszaka alapján dől el; a határ az aktuális hónap első
                        napja ({data.cutoff_date}). Az összegek nettók, az óradíj a projektből jön.
                    </p>
                </>
            )}
        </div>
    );
}
