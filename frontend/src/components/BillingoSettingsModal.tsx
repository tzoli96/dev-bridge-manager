// frontend/src/components/BillingoSettingsModal.tsx
import { useState, useEffect } from 'react'
import { BillingoSettingsService } from '@/services/billingoSettingsService'

interface BillingoSettingsModalProps {
    isOpen: boolean
    onClose: () => void
}

export default function BillingoSettingsModal({ isOpen, onClose }: BillingoSettingsModalProps) {
    const [apiKey, setApiKey] = useState('')
    const [blockId, setBlockId] = useState('')
    const [currentMaskedKey, setCurrentMaskedKey] = useState('')
    const [loading, setLoading] = useState(false)
    const [fetching, setFetching] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)

    useEffect(() => {
        if (isOpen) {
            setFetching(true)
            setSuccess(false)
            setError(null)
            BillingoSettingsService.getSettings()
                .then((settings) => {
                    setCurrentMaskedKey(settings.api_key_masked)
                    setBlockId(settings.block_id)
                })
                .catch((err) => setError(err.message))
                .finally(() => setFetching(false))
        }
    }, [isOpen])

    const handleClose = () => {
        if (!loading) {
            setApiKey('')
            setBlockId('')
            setError(null)
            setSuccess(false)
            onClose()
        }
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()

        if (!apiKey.trim()) {
            setError('API key is required to save')
            return
        }

        try {
            setLoading(true)
            setError(null)

            const updated = await BillingoSettingsService.updateSettings({
                api_key: apiKey.trim(),
                block_id: blockId.trim()
            })

            setCurrentMaskedKey(updated.api_key_masked)
            setApiKey('')
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
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Billingo Settings</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                {fetching ? (
                    <div className="text-sm text-muted-foreground py-4">Loading...</div>
                ) : (
                    <form onSubmit={handleSubmit} className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {error}
                            </div>
                        )}
                        {success && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Settings saved successfully
                            </div>
                        )}

                        <div>
                            <label htmlFor="api_key" className="block text-sm font-medium text-foreground mb-1">
                                API Key {currentMaskedKey && `(current: ${currentMaskedKey})`}
                            </label>
                            <input
                                type="password"
                                id="api_key"
                                value={apiKey}
                                onChange={(e) => setApiKey(e.target.value)}
                                placeholder="Enter new API key to change it"
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="block_id" className="block text-sm font-medium text-foreground mb-1">
                                Block ID
                            </label>
                            <input
                                type="text"
                                id="block_id"
                                value={blockId}
                                onChange={(e) => setBlockId(e.target.value)}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div className="flex space-x-3 pt-4">
                            <button
                                type="button"
                                onClick={handleClose}
                                disabled={loading}
                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                            >
                                Close
                            </button>
                            <button
                                type="submit"
                                disabled={loading || !apiKey.trim()}
                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                            >
                                {loading ? 'Saving...' : 'Save'}
                            </button>
                        </div>
                    </form>
                )}
            </div>
        </div>
    )
}
