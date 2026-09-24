// frontend/src/app/dashboard/admin/job-search/page.tsx
'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { ArrowLeft } from 'lucide-react'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { Button } from '@/components/ui/button'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import { JobSearchService, JobSearchProfile, JobMatch } from '@/services/jobSearchService'

function scoreBadgeClass(score: number): string {
    if (score >= 70) return 'bg-success/10 text-success'
    if (score >= 40) return 'bg-amber-500/10 text-amber-700'
    return 'bg-muted text-muted-foreground'
}

function statusLabel(status: JobMatch['status']): string {
    switch (status) {
        case 'new': return 'Új'
        case 'reviewed': return 'Átnézve'
        case 'dismissed': return 'Elutasítva'
        case 'applied': return 'Jelentkezve'
    }
}

export default function JobSearchPage() {
    return (
        <ProtectedRoute role="super_admin">
            <JobSearchPageContent />
        </ProtectedRoute>
    )
}

function JobSearchPageContent() {
    const [, setProfile] = useState<JobSearchProfile | null>(null)
    const [cvText, setCvText] = useState('')
    const [skills, setSkills] = useState('')
    const [preferences, setPreferences] = useState('')
    const [profileLoading, setProfileLoading] = useState(true)
    const [profileError, setProfileError] = useState<string | null>(null)
    const [savingProfile, setSavingProfile] = useState(false)
    const [profileSaved, setProfileSaved] = useState(false)

    const [matches, setMatches] = useState<JobMatch[]>([])
    const [matchesLoading, setMatchesLoading] = useState(true)
    const [matchesError, setMatchesError] = useState<string | null>(null)

    const [scanning, setScanning] = useState(false)
    const [scanMessage, setScanMessage] = useState<string | null>(null)

    const [manualUrl, setManualUrl] = useState('')
    const [manualFields, setManualFields] = useState<{ title: string; company: string; location: string; description: string } | null>(null)
    const [manualError, setManualError] = useState<string | null>(null)
    const [addingManual, setAddingManual] = useState(false)

    const loadProfile = () => {
        setProfileLoading(true)
        setProfileError(null)
        JobSearchService.getProfile()
            .then((p) => {
                setProfile(p)
                setCvText(p.cv_text)
                setSkills(p.skills)
                setPreferences(p.preferences)
            })
            .catch((err) => setProfileError(err.message))
            .finally(() => setProfileLoading(false))
    }

    const loadMatches = () => {
        setMatchesLoading(true)
        setMatchesError(null)
        JobSearchService.listMatches()
            .then(setMatches)
            .catch((err) => setMatchesError(err.message))
            .finally(() => setMatchesLoading(false))
    }

    useEffect(() => {
        loadProfile()
        loadMatches()
    }, [])

    const handleSaveProfile = async (e: React.FormEvent) => {
        e.preventDefault()
        setSavingProfile(true)
        setProfileError(null)
        setProfileSaved(false)
        try {
            const updated = await JobSearchService.updateProfile({ cv_text: cvText, skills, preferences })
            setProfile(updated)
            setProfileSaved(true)
        } catch (err) {
            setProfileError(err instanceof Error ? err.message : 'Hiba történt a profil mentése közben.')
        } finally {
            setSavingProfile(false)
        }
    }

    const handleScanNow = async () => {
        setScanning(true)
        setScanMessage(null)
        try {
            const { newListings, newMatches } = await JobSearchService.scanNow()
            setScanMessage(`${newListings} új hirdetés, ${newMatches} új értékelés.`)
            loadMatches()
        } catch (err) {
            setScanMessage(err instanceof Error ? err.message : 'Hiba történt a keresés közben.')
        } finally {
            setScanning(false)
        }
    }

    const handleAddManual = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!manualUrl.trim()) return
        setAddingManual(true)
        setManualError(null)
        try {
            await JobSearchService.addManualListing(
                manualFields ? { url: manualUrl.trim(), ...manualFields } : { url: manualUrl.trim() }
            )
            setManualUrl('')
            setManualFields(null)
            loadMatches()
        } catch (err) {
            const message = err instanceof Error ? err.message : 'Hiba történt a hirdetés hozzáadása közben.'
            setManualError(message)
            if (!manualFields) {
                setManualFields({ title: '', company: '', location: '', description: '' })
            }
        } finally {
            setAddingManual(false)
        }
    }

    const handleUpdateStatus = async (id: number, status: 'reviewed' | 'dismissed') => {
        try {
            await JobSearchService.updateMatchStatus(id, status)
            setMatches((prev) => prev.map((m) => (m.id === id ? { ...m, status } : m)))
        } catch {
            // Best-effort - the list keeps showing the stale status if this
            // fails, and the user can retry the action.
        }
    }

    return (
        <div className="p-6 space-y-6 max-w-4xl mx-auto">
            <Link href="/dashboard/admin" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
                <ArrowLeft size={14} /> Vissza az adminisztrációhoz
            </Link>

            <div>
                <h1 className="text-lg font-semibold text-foreground">Álláskeresés</h1>
                <p className="text-muted-foreground text-sm">profession.hu automatikus figyelése és AI-alapú illeszkedés-értékelés</p>
            </div>

            <section className="bg-card rounded-xl shadow-sm border border-border p-6">
                <h2 className="font-medium text-foreground mb-4">CV és preferenciák</h2>
                {profileLoading ? (
                    <LoadingState message="Betöltés..." />
                ) : (
                    <form onSubmit={handleSaveProfile} className="space-y-4">
                        {profileError && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {profileError}
                            </div>
                        )}
                        {profileSaved && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Profil elmentve
                            </div>
                        )}
                        <div>
                            <label htmlFor="cv_text" className="block text-sm font-medium text-foreground mb-1">CV szövege</label>
                            <textarea
                                id="cv_text"
                                value={cvText}
                                onChange={(e) => setCvText(e.target.value)}
                                rows={6}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={savingProfile}
                            />
                        </div>
                        <div>
                            <label htmlFor="skills" className="block text-sm font-medium text-foreground mb-1">Készségek</label>
                            <textarea
                                id="skills"
                                value={skills}
                                onChange={(e) => setSkills(e.target.value)}
                                rows={3}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={savingProfile}
                            />
                        </div>
                        <div>
                            <label htmlFor="preferences" className="block text-sm font-medium text-foreground mb-1">Preferenciák</label>
                            <textarea
                                id="preferences"
                                value={preferences}
                                onChange={(e) => setPreferences(e.target.value)}
                                rows={3}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={savingProfile}
                            />
                        </div>
                        <Button type="submit" disabled={savingProfile}>
                            {savingProfile ? 'Mentés...' : 'Mentés'}
                        </Button>
                    </form>
                )}
            </section>

            <section className="bg-card rounded-xl shadow-sm border border-border p-6 space-y-3">
                <h2 className="font-medium text-foreground">Keresés</h2>
                <Button onClick={handleScanNow} loading={scanning}>
                    Keresés most
                </Button>
                {scanMessage && <p className="text-sm text-muted-foreground">{scanMessage}</p>}
            </section>

            <section className="bg-card rounded-xl shadow-sm border border-border p-6 space-y-3">
                <h2 className="font-medium text-foreground">Hirdetés hozzáadása linkkel</h2>
                <form onSubmit={handleAddManual} className="space-y-3">
                    {manualError && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {manualError}
                        </div>
                    )}
                    <input
                        type="url"
                        value={manualUrl}
                        onChange={(e) => setManualUrl(e.target.value)}
                        placeholder="https://..."
                        className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        disabled={addingManual}
                        required
                    />
                    {manualFields && (
                        <div className="space-y-2 border border-border rounded-lg p-3">
                            <p className="text-xs text-muted-foreground">Az adatok automatikus kinyerése nem sikerült - add meg kézzel:</p>
                            <input
                                type="text"
                                value={manualFields.title}
                                onChange={(e) => setManualFields({ ...manualFields, title: e.target.value })}
                                placeholder="Munkakör"
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                            <input
                                type="text"
                                value={manualFields.company}
                                onChange={(e) => setManualFields({ ...manualFields, company: e.target.value })}
                                placeholder="Cég"
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                            <input
                                type="text"
                                value={manualFields.location}
                                onChange={(e) => setManualFields({ ...manualFields, location: e.target.value })}
                                placeholder="Helyszín"
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                            <textarea
                                value={manualFields.description}
                                onChange={(e) => setManualFields({ ...manualFields, description: e.target.value })}
                                placeholder="Leírás"
                                rows={4}
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                        </div>
                    )}
                    <Button type="submit" loading={addingManual}>
                        Hirdetés hozzáadása
                    </Button>
                </form>
            </section>

            <section className="space-y-3">
                <h2 className="font-medium text-foreground">Találatok</h2>
                {matchesLoading ? (
                    <LoadingState message="Betöltés..." />
                ) : matchesError ? (
                    <ErrorState error={matchesError} onRetry={loadMatches} />
                ) : matches.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs még egyetlen hirdetés sem.</p>
                ) : (
                    <div className="space-y-3">
                        {matches.map((match) => (
                            <div key={match.id} className="bg-card rounded-xl shadow-sm border border-border p-4 space-y-2">
                                <div className="flex items-start justify-between gap-3">
                                    <div>
                                        <h3 className="font-medium text-foreground">{match.job_listing.title}</h3>
                                        <p className="text-sm text-muted-foreground">{match.job_listing.company} - {match.job_listing.location}</p>
                                    </div>
                                    <div className="flex items-center gap-2 shrink-0">
                                        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${scoreBadgeClass(match.score)}`}>
                                            {match.score}
                                        </span>
                                        <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                                            {statusLabel(match.status)}
                                        </span>
                                    </div>
                                </div>
                                <p className="text-sm text-foreground">{match.reasoning}</p>
                                <div className="flex flex-wrap gap-2 pt-1">
                                    <a href={match.job_listing.external_url} target="_blank" rel="noopener noreferrer">
                                        <Button type="button" variant="outline" size="sm">Hirdetés megnyitása</Button>
                                    </a>
                                    {match.status !== 'dismissed' && match.status !== 'applied' && (
                                        <Button type="button" variant="danger" size="sm" onClick={() => handleUpdateStatus(match.id, 'dismissed')}>
                                            Elutasítás
                                        </Button>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </section>
        </div>
    )
}
