'use client';

import React from 'react';
import { ScenarioResult, ScenarioMonth } from '@/services/profitabilityService';
import { formatHuf } from '@/utils/formatHuf';

const formatHours = (v: number) => `${v.toFixed(1).replace('.', ',')} ó`;
const formatPercentValue = (v: number) => String(v).replace('.', ',');
const formatPercent = (v: number | null) => (v === null ? '—' : `${Math.round(v)}%`);

interface Row {
    label: string;
    base: string;
    scenario: string;
    delta: string | null;
    // positive = better for the owner, used only for the colour of the delta
    better: boolean | null;
    emphasis?: boolean;
}

function money(label: string, base: number, scenario: number, opts: { costLike?: boolean; emphasis?: boolean } = {}): Row {
    const diff = scenario - base;
    const rounded = Math.round(diff);
    return {
        label,
        base: formatHuf(base),
        scenario: formatHuf(scenario),
        delta: rounded === 0 ? '—' : `${rounded > 0 ? '+' : ''}${rounded.toLocaleString('hu-HU')} Ft`,
        better: rounded === 0 ? null : opts.costLike ? diff < 0 : diff > 0,
        emphasis: opts.emphasis,
    };
}

function buildRows(base: ScenarioMonth, scenario: ScenarioMonth): Row[] {
    const rows: Row[] = [money('Bevétel', base.revenue, scenario.revenue)];
    rows.push(money('Fix költségek', base.fixed_costs, scenario.fixed_costs, { costLike: true }));
    // Both months are priced with the same parameter set, so the items line up by index.
    scenario.revenue_items.forEach((item, i) =>
        rows.push(money(`${item.label} (${formatPercentValue(item.percent)}% a bevételből)`, base.revenue_items[i]?.amount ?? 0, item.amount, { costLike: true }))
    );
    rows.push(money('Eredmény a költségek után', base.result_after_costs, scenario.result_after_costs));
    scenario.after_costs_items.forEach((item, i) =>
        rows.push(
            money(`${item.label} (${formatPercentValue(item.percent)}% az eredményből)`, base.after_costs_items[i]?.amount ?? 0, item.amount, { costLike: true })
        )
    );
    rows.push(money('Nettó nyereség / hó', base.net_profit, scenario.net_profit, { emphasis: true }));

    const hoursDiff = scenario.required_hours - base.required_hours;
    rows.push({
        label: 'Szükséges órák / hó',
        base: formatHours(base.required_hours),
        scenario: formatHours(scenario.required_hours),
        delta: Math.abs(hoursDiff) < 0.05 ? '—' : `${hoursDiff > 0 ? '+' : ''}${hoursDiff.toFixed(1).replace('.', ',')} ó`,
        better: null,
    });
    rows.push({
        label: `Kihasználtság (kapacitás: ${formatHours(scenario.capacity_hours)})`,
        base: formatPercent(base.utilization),
        scenario: formatPercent(scenario.utilization),
        delta: null,
        better: null,
    });
    const basePerHour = base.net_profit_per_capacity_hour;
    const perHour = scenario.net_profit_per_capacity_hour;
    if (basePerHour !== null && perHour !== null) {
        rows.push(money('Nettó nyereség / kapacitásóra', basePerHour, perHour, { emphasis: true }));
    }
    return rows;
}

export default function ScenarioResultPanel({ result }: { result: ScenarioResult }) {
    const rows = buildRows(result.baseline, result.monthly);
    const horizonDelta = result.horizon.net_profit - result.baseline_horizon.net_profit;
    const roundedHorizonDelta = Math.round(horizonDelta);

    return (
        <div>
            {result.monthly.overloaded && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                    A forgatókönyv {formatHours(result.monthly.required_hours)} havi munkát igényel, ami meghaladja a megadott{' '}
                    {formatHours(result.monthly.capacity_hours)} kapacitást.
                </div>
            )}

            {result.warnings.map((w, i) => (
                <div key={`${i}-${w}`} className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm mb-3">
                    {w}
                </div>
            ))}

            <div className="overflow-x-auto bg-card border border-border rounded-lg">
                <table className="w-full text-sm">
                    <thead className="text-left text-muted-foreground border-b border-border">
                        <tr>
                            <th className="p-3">Havi eredmény</th>
                            <th className="p-3 text-right">Alaphelyzet</th>
                            <th className="p-3 text-right">Forgatókönyv</th>
                            <th className="p-3 text-right">Változás</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row, i) => (
                            <tr key={`${i}-${row.label}`} className="border-b border-border last:border-0">
                                <td className={`p-3 ${row.emphasis ? 'font-medium text-foreground' : 'text-foreground'}`}>{row.label}</td>
                                <td className="p-3 text-right">{row.base}</td>
                                <td className={`p-3 text-right ${row.emphasis ? 'font-medium' : ''}`}>{row.scenario}</td>
                                <td
                                    className={`p-3 text-right ${
                                        row.better === null ? 'text-muted-foreground' : row.better ? 'text-success' : 'text-destructive'
                                    }`}
                                >
                                    {row.delta ?? '—'}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>

            <p className="text-sm text-foreground mt-3">
                {result.months.length} hónap alatt a nettó nyereség{' '}
                <span className="font-medium">{formatHuf(result.horizon.net_profit)}</span> (alaphelyzet:{' '}
                {formatHuf(result.baseline_horizon.net_profit)}, különbség:{' '}
                {roundedHorizonDelta === 0 ? (
                    <span className="text-muted-foreground">—</span>
                ) : (
                    <span className={roundedHorizonDelta > 0 ? 'text-success' : 'text-destructive'}>
                        {roundedHorizonDelta > 0 ? '+' : ''}
                        {roundedHorizonDelta.toLocaleString('hu-HU')} Ft
                    </span>
                )}
                ).
            </p>

            <details className="mt-3 text-xs text-muted-foreground">
                <summary className="cursor-pointer">Mivel számoltunk?</summary>
                <ul className="mt-2 space-y-1">
                    <li>
                        {result.basis.baseline_months.length > 0
                            ? `Alap: az utolsó ${result.basis.baseline_months.length} teljes hónap (${result.basis.baseline_months.join(', ')}) rendszeres (nem fix áras) számláinak havi átlaga, nettó összegekkel; az órák a naplózott és a becsült levelezési és megbeszélési órák havi átlaga.`
                            : 'Alap: nincs rendszeres számlázási előzmény az alapidőszakban.'}
                    </li>
                    <li>Paraméterkészlet: {result.basis.parameter_set_name}</li>
                    {result.basis.fixed_monthly_costs.map((c, i) => (
                        <li key={`c${i}`}>
                            {c.label}: {formatHuf(c.amount)} / hó
                        </li>
                    ))}
                    {result.basis.percent_items.map((p, i) => (
                        <li key={`p${i}`}>
                            {p.label}: {formatPercentValue(p.percent)}% ({p.base === 'revenue' ? 'a bevételből' : 'a költségek utáni eredményből'})
                        </li>
                    ))}
                    <li>
                        Kapacitás: {formatHours(result.basis.capacity_hours)} / hó. A havi eredmény minden vizsgált hónapra azonos; a
                        szerződések lejárata nincs figyelembe véve. A százalékos tételek az eredményen nem halmozódnak, és
                        negatív eredményre nem számolódik tétel.
                    </li>
                    {result.low_data && <li>Kevés a rendszeres számlázási előzmény, a számok tájékoztató jellegűek.</li>}
                </ul>
            </details>
        </div>
    );
}
