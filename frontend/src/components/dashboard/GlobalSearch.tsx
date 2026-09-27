'use client'

import { useEffect, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Search, Users, Briefcase, ListTodo } from 'lucide-react'
import { SearchService, SearchResponse } from '@/services/searchService'

export default function GlobalSearch() {
    const router = useRouter()
    const containerRef = useRef<HTMLDivElement>(null)
    const [query, setQuery] = useState('')
    const [results, setResults] = useState<SearchResponse | null>(null)
    const [open, setOpen] = useState(false)
    const [loading, setLoading] = useState(false)

    useEffect(() => {
        const trimmed = query.trim()
        if (trimmed.length < 2) {
            setResults(null)
            setLoading(false)
            return
        }

        setLoading(true)
        const timeout = setTimeout(() => {
            SearchService.search(trimmed)
                .then((res) => setResults(res))
                .catch(() => setResults(null))
                .finally(() => setLoading(false))
        }, 300)

        return () => clearTimeout(timeout)
    }, [query])

    useEffect(() => {
        function handleClickOutside(e: MouseEvent) {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setOpen(false)
            }
        }
        document.addEventListener('mousedown', handleClickOutside)
        return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [])

    function goTo(path: string) {
        setOpen(false)
        setQuery('')
        setResults(null)
        router.push(path)
    }

    const hasResults =
        !!results && (results.clients.length > 0 || results.projects.length > 0 || results.tasks.length > 0)

    return (
        <div ref={containerRef} className="relative">
            <div className="relative">
                <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                <input
                    type="text"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    onFocus={() => setOpen(true)}
                    placeholder="Keresés ügyfelek, projektek, feladatok között..."
                    className="w-full pl-9 pr-3 py-2 text-sm bg-background border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                />
            </div>

            {open && query.trim().length >= 2 && (
                <div className="absolute top-full mt-1 w-full bg-card border border-border rounded-lg shadow-lg z-50 max-h-96 overflow-y-auto">
                    {loading && (
                        <div className="px-3 py-3 text-sm text-muted-foreground">Keresés...</div>
                    )}
                    {!loading && !hasResults && (
                        <div className="px-3 py-3 text-sm text-muted-foreground">Nincs találat.</div>
                    )}
                    {!loading && results && results.clients.length > 0 && (
                        <div className="py-1">
                            <div className="px-3 pt-1.5 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Ügyfelek</div>
                            {results.clients.map((c) => (
                                <button
                                    key={c.id}
                                    onClick={() => goTo(`/dashboard/clients/${c.id}`)}
                                    className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-accent"
                                >
                                    <Users size={14} className="text-muted-foreground shrink-0" />
                                    <span className="truncate">{c.name}</span>
                                </button>
                            ))}
                        </div>
                    )}
                    {!loading && results && results.projects.length > 0 && (
                        <div className="py-1 border-t border-border">
                            <div className="px-3 pt-1.5 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Projektek</div>
                            {results.projects.map((p) => (
                                <button
                                    key={p.id}
                                    onClick={() => goTo(`/dashboard/board/${p.id}`)}
                                    className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-accent"
                                >
                                    <Briefcase size={14} className="text-muted-foreground shrink-0" />
                                    <span className="truncate">{p.name}</span>
                                </button>
                            ))}
                        </div>
                    )}
                    {!loading && results && results.tasks.length > 0 && (
                        <div className="py-1 border-t border-border">
                            <div className="px-3 pt-1.5 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Feladatok</div>
                            {results.tasks.map((t) => (
                                <button
                                    key={t.id}
                                    onClick={() => goTo(`/dashboard/board/${t.project_id}`)}
                                    className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-accent"
                                >
                                    <ListTodo size={14} className="text-muted-foreground shrink-0" />
                                    <span className="truncate">{t.title}</span>
                                    <span className="ml-auto text-xs text-muted-foreground truncate max-w-[100px]">{t.project_name}</span>
                                </button>
                            ))}
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
