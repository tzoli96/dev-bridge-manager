'use client';

import React from 'react';
import { profitabilityService, ProfitabilityForecast } from '@/services/profitabilityService';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { formatHuf } from '@/utils/formatHuf';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

// Rounds a maximum up to 1, 2, 5 or 10 x a power of ten so the axis ticks are readable.
function niceMax(value: number): number {
    if (value <= 0) return 1;
    const exp = Math.pow(10, Math.floor(Math.log10(value)));
    const f = value / exp;
    const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10;
    return nice * exp;
}

function compactHuf(value: number): string {
    if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1).replace(/\.0$/, '').replace('.', ',')} M Ft`;
    if (value >= 1_000) return `${Math.round(value / 1_000)} e Ft`;
    return `${Math.round(value)} Ft`;
}

interface ForecastChartProps {
    months: string[];
    committed: number[];
    dependent: number[];
}

// Stacked bars per month: solid = committed, hatched = depends on renewal.
// The hatch (not only the colour) separates the two series; the table below
// the chart repeats every value as text.
function ForecastChart({ months, committed, dependent }: ForecastChartProps) {
    const W = 640;
    const H = 260;
    const margin = { top: 12, right: 12, bottom: 28, left: 64 };
    const plotW = W - margin.left - margin.right;
    const plotH = H - margin.top - margin.bottom;

    const totals = months.map((_, i) => (committed[i] ?? 0) + (dependent[i] ?? 0));
    const max = niceMax(Math.max(0, ...totals));
    const y = (v: number) => margin.top + plotH - (v / max) * plotH;
    const band = plotW / Math.max(months.length, 1);
    const barW = Math.min(56, band * 0.6);
    const ticks = [0, 0.5, 1].map((f) => f * max);

    return (
        <svg
            viewBox={`0 0 ${W} ${H}`}
            role="img"
            aria-label={`Bevétel-előrejelzés ${months[0] ?? ''} és ${months[months.length - 1] ?? ''} között, havonta`}
            className="w-full h-auto"
        >
            <defs>
                <pattern id="dependent-hatch" patternUnits="userSpaceOnUse" width="6" height="6" patternTransform="rotate(45)">
                    <rect width="6" height="6" className="fill-primary/15" />
                    <line x1="0" y1="0" x2="0" y2="6" strokeWidth="2" className="stroke-primary" />
                </pattern>
            </defs>

            {ticks.map((t) => (
                <g key={t}>
                    <line x1={margin.left} x2={W - margin.right} y1={y(t)} y2={y(t)} className="stroke-border" strokeWidth="1" />
                    <text x={margin.left - 8} y={y(t) + 3} textAnchor="end" className="fill-muted-foreground" fontSize="10">
                        {compactHuf(t)}
                    </text>
                </g>
            ))}

            {months.map((m, i) => {
                const c = committed[i] ?? 0;
                const d = dependent[i] ?? 0;
                const x = margin.left + band * i + (band - barW) / 2;
                return (
                    <g key={m}>
                        <title>{`${m}: biztos ${formatHuf(c)}, megújítástól függő ${formatHuf(d)}`}</title>
                        <rect x={x} y={y(c)} width={barW} height={Math.max(y(0) - y(c), 0)} className="fill-primary" />
                        <rect
                            x={x}
                            y={y(c + d)}
                            width={barW}
                            height={Math.max(y(c) - y(c + d), 0)}
                            fill="url(#dependent-hatch)"
                            className="stroke-primary"
                            strokeWidth="1"
                        />
                        <text x={x + barW / 2} y={H - 8} textAnchor="middle" className="fill-muted-foreground" fontSize="10">
                            {m}
                        </text>
                    </g>
                );
            })}
        </svg>
    );
}

export default function ForecastTab() {
    const [months, setMonths] = React.useState(6);
    const [data, setData] = React.useState<ProfitabilityForecast | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            setData(await profitabilityService.forecast(months));
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, [months]);

    React.useEffect(() => {
        load();
    }, [load]);

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <p className="text-xs text-muted-foreground max-w-2xl">
                    Az előrejelzés az utolsó {data?.baseline_months.length || 3} teljes hónap rendszeres
                    (nem fix áras) számláinak ügyfélenkénti átlagát vetíti előre, az aktuális hónappal kezdve. A
                    szerződés lejárata utáni hónapok bevétele a „megújítástól függő” sávba kerül.
                </p>
                <select
                    value={months}
                    onChange={(e) => setMonths(Number(e.target.value))}
                    className="border border-border rounded-md bg-background text-sm px-2 py-1.5"
                >
                    {[3, 6].map((m) => (
                        <option key={m} value={m}>
                            Következő {m} hónap
                        </option>
                    ))}
                </select>
            </div>

            {loading && <LoadingState message="Előrejelzés betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={load} />}

            {!loading && !error && data && (
                <>
                    {data.warnings.map((w) => (
                        <div key={w} className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm mb-4">
                            {w}
                        </div>
                    ))}

                    {data.clients.length === 0 ? (
                        <EmptyState
                            icon="files"
                            title="Nincs rendszeres bevétel az alapidőszakban"
                            description="Az előrejelzéshez kiállított, nem fix áras számla kell az utolsó 3 teljes hónapból."
                        />
                    ) : (
                        <>
                            <div className="bg-card border border-border rounded-lg p-4 mb-4">
                                <ForecastChart months={data.months} committed={data.committed} dependent={data.dependent} />
                                <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                                    <span className="inline-flex items-center gap-1.5">
                                        <span className="inline-block w-3 h-3 bg-primary" /> Biztos
                                    </span>
                                    <span className="inline-flex items-center gap-1.5">
                                        <svg width="12" height="12" aria-hidden="true">
                                            <rect width="12" height="12" className="fill-primary/15 stroke-primary" strokeWidth="1" />
                                            <line x1="0" y1="12" x2="12" y2="0" className="stroke-primary" strokeWidth="1.5" />
                                        </svg>
                                        Megújítástól függő
                                    </span>
                                </div>
                            </div>

                            <section className="mb-6">
                                <h2 className="text-lg font-semibold text-foreground mb-3">Havi összesítő</h2>
                                <div className="overflow-x-auto bg-card border border-border rounded-lg">
                                    <table className="w-full text-sm">
                                        <thead className="text-left text-muted-foreground border-b border-border">
                                            <tr>
                                                <th className="p-3">Hónap</th>
                                                <th className="p-3 text-right">Biztos</th>
                                                <th className="p-3 text-right">Megújítástól függő</th>
                                                <th className="p-3 text-right">Összesen</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {data.months.map((m, i) => (
                                                <tr key={m} className="border-b border-border last:border-0">
                                                    <td className="p-3 text-foreground">{m}</td>
                                                    <td className="p-3 text-right">{formatHuf(data.committed[i])}</td>
                                                    <td className="p-3 text-right">{formatHuf(data.dependent[i])}</td>
                                                    <td className="p-3 text-right font-medium">
                                                        {formatHuf(data.committed[i] + data.dependent[i])}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </section>

                            <section>
                                <h2 className="text-lg font-semibold text-foreground mb-3">Ügyfelek</h2>
                                <div className="overflow-x-auto bg-card border border-border rounded-lg">
                                    <table className="w-full text-sm">
                                        <thead className="text-left text-muted-foreground border-b border-border">
                                            <tr>
                                                <th className="p-3">Ügyfél</th>
                                                <th className="p-3 text-right">Havi átlag</th>
                                                <th className="p-3 text-right">Szerződés vége</th>
                                                <th className="p-3 text-right">Biztos ({data.months.length} hó)</th>
                                                <th className="p-3 text-right">Megújítástól függő</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {data.clients.map((c) => (
                                                <tr key={c.client_id} className="border-b border-border last:border-0">
                                                    <td className="p-3 text-foreground">
                                                        {c.name}
                                                        {c.months_with_data < data.baseline_months.length && (
                                                            <span
                                                                className="ml-2 text-xs bg-muted text-muted-foreground rounded-full px-2 py-0.5"
                                                                title={`Csak ${c.months_with_data} hónapban volt számlázva az alapidőszakban`}
                                                            >
                                                                kevés adat
                                                            </span>
                                                        )}
                                                    </td>
                                                    <td className="p-3 text-right">{formatHuf(c.monthly_average)}</td>
                                                    <td className="p-3 text-right">{c.contract_end_month ?? '—'}</td>
                                                    <td className="p-3 text-right">
                                                        {formatHuf(c.committed.reduce((a, b) => a + b, 0))}
                                                    </td>
                                                    <td className="p-3 text-right">
                                                        {formatHuf(c.dependent.reduce((a, b) => a + b, 0))}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </section>
                        </>
                    )}
                </>
            )}
        </div>
    );
}
