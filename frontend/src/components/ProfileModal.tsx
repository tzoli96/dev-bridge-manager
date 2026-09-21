// frontend/src/components/ProfileModal.tsx
import { useState, useEffect } from 'react'
import { ProfileService, ProfileSample } from '@/services/profileService'

interface ProfileModalProps {
    isOpen: boolean
    onClose: () => void
}

const SOFT_SAMPLE_WARNING_THRESHOLD = 8

export default function ProfileModal({ isOpen, onClose }: ProfileModalProps) {
    const [background, setBackground] = useState('')
    const [expertise, setExpertise] = useState('')
    const [toneRules, setToneRules] = useState('')
    const [samples, setSamples] = useState<ProfileSample[]>([])
    const [loading, setLoading] = useState(false)
    const [fetching, setFetching] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)

    useEffect(() => {
        if (isOpen) {
            setFetching(true)
            setSuccess(false)
            setError(null)
            ProfileService.getProfile()
                .then((profile) => {
                    setBackground(profile.background)
                    setExpertise(profile.expertise)
                    setToneRules(profile.tone_rules)
                    setSamples(profile.samples)
                })
                .catch((err) => setError(err.message))
                .finally(() => setFetching(false))
        }
    }, [isOpen])

    const handleClose = () => {
        if (!loading) {
            setError(null)
            setSuccess(false)
            onClose()
        }
    }

    const addSample = () => {
        setSamples(prev => [...prev, { label: '', content: '' }])
    }

    const updateSample = (index: number, field: 'label' | 'content', value: string) => {
        setSamples(prev => prev.map((s, i) => (i === index ? { ...s, [field]: value } : s)))
    }

    const removeSample = (index: number) => {
        setSamples(prev => prev.filter((_, i) => i !== index))
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        try {
            setLoading(true)
            setError(null)
            const updated = await ProfileService.updateProfile({
                background,
                expertise,
                tone_rules: toneRules,
                samples: samples.filter(s => s.label.trim() || s.content.trim()),
            })
            setSamples(updated.samples)
            setSuccess(true)
        } catch (err: any) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    if (!isOpen) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-2xl">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">AI Profil / Perszóna</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                {fetching ? (
                    <div className="text-sm text-muted-foreground py-4">Betöltés...</div>
                ) : (
                    <form onSubmit={handleSubmit} className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {error}
                            </div>
                        )}
                        {success && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Profil elmentve
                            </div>
                        )}

                        <div>
                            <label htmlFor="background" className="block text-sm font-medium text-foreground mb-1">
                                Háttér
                            </label>
                            <textarea
                                id="background"
                                value={background}
                                onChange={(e) => setBackground(e.target.value)}
                                rows={3}
                                placeholder="Mivel foglalkozol, mi a szereped..."
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="expertise" className="block text-sm font-medium text-foreground mb-1">
                                Szakterület
                            </label>
                            <textarea
                                id="expertise"
                                value={expertise}
                                onChange={(e) => setExpertise(e.target.value)}
                                rows={3}
                                placeholder="Mihez értesz..."
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="tone_rules" className="block text-sm font-medium text-foreground mb-1">
                                Kommunikációs stílus
                            </label>
                            <textarea
                                id="tone_rules"
                                value={toneRules}
                                onChange={(e) => setToneRules(e.target.value)}
                                rows={3}
                                placeholder="Milyen hangnemben, stílusban kommunikálsz..."
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div className="space-y-2">
                            <div className="flex items-center justify-between">
                                <label className="block text-sm font-medium text-foreground">Írásminták</label>
                                <button
                                    type="button"
                                    onClick={addSample}
                                    disabled={loading}
                                    className="text-xs text-primary hover:underline disabled:opacity-50"
                                >
                                    + Új minta
                                </button>
                            </div>
                            {samples.length > SOFT_SAMPLE_WARNING_THRESHOLD && (
                                <p className="text-xs text-amber-600">
                                    {samples.length} minta van megadva - ennyi minta felett az AI nehezebben tudja kiemelni a jellemző stílust.
                                </p>
                            )}
                            {samples.map((sample, index) => (
                                <div key={index} className="border border-border rounded-lg p-3 space-y-2">
                                    <div className="flex items-center gap-2">
                                        <input
                                            type="text"
                                            value={sample.label}
                                            onChange={(e) => updateSample(index, 'label', e.target.value)}
                                            placeholder="Cím (pl. Ügyfélnek írt email)"
                                            className="flex-1 px-3 py-1.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                            disabled={loading}
                                        />
                                        <button
                                            type="button"
                                            onClick={() => removeSample(index)}
                                            disabled={loading}
                                            className="text-xs text-destructive hover:underline disabled:opacity-50"
                                        >
                                            Törlés
                                        </button>
                                    </div>
                                    <textarea
                                        value={sample.content}
                                        onChange={(e) => updateSample(index, 'content', e.target.value)}
                                        rows={3}
                                        placeholder="A minta szövege..."
                                        className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                        disabled={loading}
                                    />
                                </div>
                            ))}
                        </div>

                        <div className="flex space-x-3 pt-4">
                            <button
                                type="button"
                                onClick={handleClose}
                                disabled={loading}
                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                            >
                                Bezárás
                            </button>
                            <button
                                type="submit"
                                disabled={loading}
                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                            >
                                {loading ? 'Mentés...' : 'Mentés'}
                            </button>
                        </div>
                    </form>
                )}
            </div>
        </div>
    )
}
