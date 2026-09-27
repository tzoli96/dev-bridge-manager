'use client'

import React from 'react'
import { KanbanStallService, KanbanStallFlagWithNames } from '@/services/kanbanStallService'
import { Button } from '@/components/ui/button'

export default function StalledTasksWidget() {
    const [flags, setFlags] = React.useState<KanbanStallFlagWithNames[]>([])
    const [loading, setLoading] = React.useState(true)
    const [actingId, setActingId] = React.useState<number | null>(null)
    const [error, setError] = React.useState<string | null>(null)

    const fetchFlags = React.useCallback(() => {
        setLoading(true)
        KanbanStallService.list('pending')
            .then(setFlags)
            .catch(() => setFlags([]))
            .finally(() => setLoading(false))
    }, [])

    React.useEffect(() => {
        fetchFlags()
    }, [fetchFlags])

    const handleDismiss = async (flag: KanbanStallFlagWithNames) => {
        setError(null)
        try {
            setActingId(flag.id)
            await KanbanStallService.dismiss(flag.id)
            fetchFlags()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'A elvetés sikertelen')
        } finally {
            setActingId(null)
        }
    }

    if (!loading && flags.length === 0) {
        return null
    }

    return (
        <div className="bg-card rounded-xl shadow-sm border border-border p-6">
            <h2 className="text-lg font-semibold text-foreground mb-1">Megakadt feladatok</h2>
            <p className="text-xs text-muted-foreground mb-3">
                Feladatok, amelyek 5 vagy több napja ugyanabban az oszlopban ülnek.
            </p>
            {error && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                    {error}
                </div>
            )}
            {loading ? (
                <div className="flex items-center justify-center h-20">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                </div>
            ) : (
                <div className="bg-background border border-border rounded-lg divide-y divide-border">
                    {flags.map((flag) => (
                        <div key={flag.id} className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 px-4 py-3">
                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-medium text-foreground">
                                    {flag.task_title}
                                    {' · '}
                                    {flag.project_name}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                    {`${flag.column_title} oszlopban · ${flag.days_stalled} napja`}
                                </p>
                            </div>
                            <div className="flex-shrink-0">
                                <Button
                                    size="sm"
                                    variant="outline"
                                    loading={actingId === flag.id}
                                    onClick={() => handleDismiss(flag)}
                                >
                                    Elvetés
                                </Button>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
