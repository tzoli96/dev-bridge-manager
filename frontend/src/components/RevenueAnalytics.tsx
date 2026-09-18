'use client'

import React from 'react'
import { InvoicesService, RevenueAnalytics as RevenueAnalyticsData } from '@/services/invoicesService'
import { TrendingUp, ArrowUp, ArrowDown, Minus } from 'lucide-react'

interface Props {
    projectId?: number
    clientId?: number
    title?: string
}

const MONTH_LABELS = ['jan', 'febr', 'márc', 'ápr', 'máj', 'jún', 'júl', 'aug', 'szept', 'okt', 'nov', 'dec']

function formatMonthLabel(month: string): string {
    const idx = Number(month.split('-')[1]) - 1
    return MONTH_LABELS[idx] ?? month
}

function formatHuf(amount: number): string {
    return `${amount.toLocaleString('hu-HU')} HUF`
}

export default function RevenueAnalytics({ projectId, clientId, title = 'Bevétel elemzés' }: Props) {
    const [data, setData] = React.useState<RevenueAnalyticsData | null>(null)
    const [loading, setLoading] = React.useState(true)

    React.useEffect(() => {
        let cancelled = false
        setLoading(true)
        const request = projectId
            ? InvoicesService.getProjectRevenueAnalytics(projectId)
            : clientId
                ? InvoicesService.getClientRevenueAnalytics(clientId)
                : Promise.resolve(null)

        request
            .then((result) => { if (!cancelled) setData(result) })
            .catch(() => { if (!cancelled) setData(null) })
            .finally(() => { if (!cancelled) setLoading(false) })

        return () => { cancelled = true }
    }, [projectId, clientId])

    if (loading) {
        return <div className="text-sm text-muted-foreground">Bevétel elemzés betöltése...</div>
    }

    if (!data || data.total === 0) {
        return null
    }

    const maxMonthly = Math.max(...data.monthly.map((m) => m.amount), 1)

    // The backend only returns months that actually have revenue, so index
    // by month key rather than relying on array position for "current" /
    // "previous month" lookups.
    const now = new Date()
    const currentMonthKey = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
    const prevDate = new Date(now.getFullYear(), now.getMonth() - 1, 1)
    const prevMonthKey = `${prevDate.getFullYear()}-${String(prevDate.getMonth() + 1).padStart(2, '0')}`
    const monthlyByKey = new Map(data.monthly.map((m) => [m.month, m.amount]))
    const currentMonthAmount = monthlyByKey.get(currentMonthKey) || 0
    const prevMonthAmount = monthlyByKey.get(prevMonthKey) || 0
    const momChange = prevMonthAmount > 0 ? ((currentMonthAmount - prevMonthAmount) / prevMonthAmount) * 100 : null

    const monthlySum = data.monthly.reduce((sum, m) => sum + m.amount, 0)
    const avgMonthly = monthlySum / 12

    const bestMonth = data.monthly.reduce<{ month: string; amount: number } | null>(
        (best, m) => (!best || m.amount > best.amount ? m : best),
        null
    )

    return (
        <div className="bg-card border border-border rounded-lg p-4">
            <div className="flex items-center gap-2 mb-4">
                <TrendingUp size={16} className="text-primary" />
                <h3 className="text-sm font-semibold text-foreground">{title}</h3>
            </div>

            <div className="flex flex-wrap gap-x-8 gap-y-3 mb-5">
                <div>
                    <div className="text-xs text-muted-foreground">Összesen</div>
                    <div className="text-xl font-bold text-foreground">{formatHuf(data.total)}</div>
                </div>
                {data.yearly.map((y) => (
                    <div key={y.year}>
                        <div className="text-xs text-muted-foreground">{y.year}</div>
                        <div className="text-xl font-bold text-foreground">{formatHuf(y.amount)}</div>
                    </div>
                ))}
                <div>
                    <div className="text-xs text-muted-foreground">Aktuális hónap</div>
                    <div className="flex items-center gap-1.5">
                        <span className="text-xl font-bold text-foreground">{formatHuf(currentMonthAmount)}</span>
                        {momChange !== null && (
                            <span
                                className={[
                                    'inline-flex items-center gap-0.5 text-xs font-medium px-1.5 py-0.5 rounded-full',
                                    momChange > 0
                                        ? 'bg-success/10 text-success'
                                        : momChange < 0
                                            ? 'bg-destructive/10 text-destructive'
                                            : 'bg-muted text-muted-foreground'
                                ].join(' ')}
                            >
                                {momChange > 0 ? <ArrowUp size={11} /> : momChange < 0 ? <ArrowDown size={11} /> : <Minus size={11} />}
                                {Math.abs(momChange).toFixed(0)}%
                            </span>
                        )}
                    </div>
                </div>
                <div>
                    <div className="text-xs text-muted-foreground">Havi átlag (12 hó)</div>
                    <div className="text-xl font-bold text-foreground">{formatHuf(Math.round(avgMonthly))}</div>
                </div>
                {bestMonth && (
                    <div>
                        <div className="text-xs text-muted-foreground">Legjobb hónap</div>
                        <div className="text-xl font-bold text-foreground">
                            {formatMonthLabel(bestMonth.month)} <span className="text-sm font-medium text-muted-foreground">({formatHuf(bestMonth.amount)})</span>
                        </div>
                    </div>
                )}
            </div>

            {data.monthly.length > 0 && (
                <div>
                    <div className="text-xs text-muted-foreground mb-2">Havi bontás (utolsó 12 hónap)</div>
                    <div className="flex items-end gap-1.5 h-32">
                        {data.monthly.map((m) => (
                            <div key={m.month} className="flex-1 h-full flex flex-col items-center justify-end group relative">
                                <div className="absolute -top-6 opacity-0 group-hover:opacity-100 transition-opacity bg-foreground text-background text-[10px] px-1.5 py-0.5 rounded whitespace-nowrap pointer-events-none z-10">
                                    {formatHuf(m.amount)}
                                </div>
                                <div
                                    className="w-full bg-primary/70 group-hover:bg-primary rounded-t-sm transition-colors min-h-[2px]"
                                    style={{ height: `${(m.amount / maxMonthly) * 100}%` }}
                                />
                                <span className="text-[10px] text-muted-foreground mt-1 whitespace-nowrap">{formatMonthLabel(m.month)}</span>
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    )
}
