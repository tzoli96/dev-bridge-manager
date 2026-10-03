'use client'

import { useRouter } from 'next/navigation'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { useProjects } from '@/hooks/useProjects'
import { useModals } from '@/components/dashboard/DashboardModals'
import { useAuth } from '@/contexts/AuthContext'
import { isAdmin } from '@/utils/permissions'
import { groupProjectsByClient, projectClientNames } from '@/utils/groupProjects'
import { Button } from '@/components/ui/button'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import EmptyState from '@/components/ui/EmptyState'

const statusStyles: Record<string, string> = {
    active: 'bg-success/10 text-success',
    completed: 'bg-primary/10 text-primary',
    'on-hold': 'bg-warning/10 text-warning',
    cancelled: 'bg-muted text-muted-foreground',
}

function formatPricing(project: { pricing_type?: 'hourly' | 'fixed' | 'hobby' | ''; hourly_rate?: number | null; fixed_price?: number | null }): string {
    if (project.pricing_type === 'hourly' && project.hourly_rate) {
        return `${project.hourly_rate.toLocaleString()} HUF/hr`
    }
    if (project.pricing_type === 'fixed' && project.fixed_price) {
        return `${project.fixed_price.toLocaleString()} HUF fixed`
    }
    if (project.pricing_type === 'hobby') {
        return 'Hobbi projekt'
    }
    return '-'
}

const COLLAPSED_STORAGE_KEY = 'board-projects-collapsed-groups'

// The set of collapsed group keys is persisted; anything not listed is open.
function readCollapsed(): string[] {
    try {
        const raw = window.localStorage.getItem(COLLAPSED_STORAGE_KEY)
        const parsed: unknown = raw ? JSON.parse(raw) : []
        return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
    } catch {
        return []
    }
}

export default function BoardPage() {
    const router = useRouter()
    const { user } = useAuth()
    const { projects, loading, error, refetch } = useProjects()
    const { setShowCreateProject, setShowEditProject, setSelectedProject, setOnProjectUpdated } = useModals()
    const [collapsed, setCollapsed] = useState<string[]>([])
    const [collapsedLoaded, setCollapsedLoaded] = useState(false)

    useEffect(() => {
        setOnProjectUpdated(() => refetch)
        return () => setOnProjectUpdated(undefined)
    }, [setOnProjectUpdated, refetch])

    // localStorage is only available in the browser: read it after mount.
    useEffect(() => {
        setCollapsed(readCollapsed())
        setCollapsedLoaded(true)
    }, [])

    const groups = useMemo(() => groupProjectsByClient(projects ?? []), [projects])

    const persist = useCallback((next: string[]) => {
        setCollapsed(next)
        try {
            window.localStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify(next))
        } catch {
            // Storage unavailable (private mode / quota): the state still works for this visit.
        }
    }, [])

    const toggleGroup = (key: string) => {
        persist(collapsed.includes(key) ? collapsed.filter((k) => k !== key) : [...collapsed, key])
    }

    if (loading) return <LoadingState message="Projektek betöltése..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    const projectCount = projects?.length ?? 0
    const allCollapsed = groups.length > 0 && groups.every((group) => collapsed.includes(group.key))

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">Projektek</h1>
                    <p className="text-muted-foreground text-sm">Válassz egy projektet a boardjainak megnyitásához</p>
                </div>
                {isAdmin(user) && (
                    <Button onClick={() => setShowCreateProject(true)}>
                        Új projekt
                    </Button>
                )}
            </div>

            {projectCount === 0 ? (
                <EmptyState
                    icon="projects"
                    title="Nincs projekt"
                    description="Nincs megjeleníthető projekt."
                    action={isAdmin(user) ? {
                        label: "Első projekt létrehozása",
                        onClick: () => setShowCreateProject(true)
                    } : undefined}
                />
            ) : (
                <div className="space-y-4">
                    <div className="flex items-center justify-between">
                        <p className="text-sm text-muted-foreground">
                            {projectCount} projekt, {groups.length} csoport
                        </p>
                        <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => persist(allCollapsed ? [] : groups.map((group) => group.key))}
                        >
                            {allCollapsed ? 'Mind lenyit' : 'Mind becsuk'}
                        </Button>
                    </div>

                    {groups.map((group) => {
                        const isOpen = !collapsedLoaded || !collapsed.includes(group.key)
                        const panelId = `projects-${group.key.replace(/[^a-z0-9]+/gi, '-')}`
                        const Chevron = isOpen ? ChevronDown : ChevronRight
                        return (
                            <section key={group.key} className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
                                <button
                                    type="button"
                                    onClick={() => toggleGroup(group.key)}
                                    aria-expanded={isOpen}
                                    aria-controls={panelId}
                                    className="flex w-full items-center gap-3 bg-muted px-6 py-3 text-left transition-colors hover:bg-muted/70"
                                >
                                    <Chevron size={16} className="shrink-0 text-muted-foreground" />
                                    <span className="font-medium text-foreground">{group.label}</span>
                                    <span className="text-sm text-muted-foreground">
                                        {group.projects.length} projekt
                                    </span>
                                </button>

                                {isOpen && (
                                    <div id={panelId} className="overflow-x-auto">
                                        <table className="min-w-full divide-y divide-border">
                                            <thead className="bg-muted">
                                                <tr>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Projekt</th>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Árazás</th>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Állapot</th>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Létrehozta</th>
                                                    <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Műveletek</th>
                                                </tr>
                                            </thead>
                                            <tbody className="bg-card divide-y divide-border">
                                                {group.projects.map(project => (
                                                    <tr key={project.id} className="hover:bg-muted/50">
                                                        <td className="px-6 py-4">
                                                            <div className="text-sm font-medium text-foreground">{project.name}</div>
                                                            {group.kind === 'multiple' && (
                                                                <div className="text-xs text-muted-foreground">{projectClientNames(project)}</div>
                                                            )}
                                                            <div className="text-sm text-muted-foreground line-clamp-1">{project.description}</div>
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">
                                                            {formatPricing(project)}
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap">
                                                            <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${statusStyles[project.status] || statusStyles.active}`}>
                                                                {project.status}
                                                            </span>
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                            {project.created_by_name || '-'}
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm space-x-4">
                                                            {isAdmin(user) && (
                                                                <button
                                                                    onClick={() => {
                                                                        setSelectedProject(project)
                                                                        setShowEditProject(true)
                                                                    }}
                                                                    className="text-muted-foreground hover:text-foreground font-medium"
                                                                >
                                                                    Szerkesztés
                                                                </button>
                                                            )}
                                                            <button
                                                                onClick={() => router.push(`/dashboard/board/${project.id}`)}
                                                                className="text-primary hover:text-primary/80 font-medium"
                                                            >
                                                                Megnyitás
                                                            </button>
                                                        </td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                )}
                            </section>
                        )
                    })}
                </div>
            )}
        </div>
    )
}
