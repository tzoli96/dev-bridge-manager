// frontend/src/components/BillingoSettingsModal.tsx
import { useState, useEffect } from 'react'
import { BillingoSettingsService, BillingoDocumentBlock } from '@/services/billingoSettingsService'

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
    const [blocks, setBlocks] = useState<BillingoDocumentBlock[]>([])
    const [blocksLoading, setBlocksLoading] = useState(false)
    const [blocksError, setBlocksError] = useState<string | null>(null)
    const [defaultUnit, setDefaultUnit] = useState('')
    const [defaultUnitPriceType, setDefaultUnitPriceType] = useState<'net' | 'gross'>('net')

    useEffect(() => {
        if (isOpen) {
            setFetching(true)
            setSuccess(false)
            setError(null)
            setBlocks([])
            setBlocksError(null)
            BillingoSettingsService.getSettings()
                .then((settings) => {
                    setCurrentMaskedKey(settings.api_key_masked)
                    setBlockId(settings.block_id)
                    setDefaultUnit(settings.default_unit || 'db')
                    setDefaultUnitPriceType(settings.default_unit_price_type || 'net')
                    if (settings.api_key_masked) {
                        loadBlocks()
                    }
                })
                .catch((err) => setError(err.message))
                .finally(() => setFetching(false))
        }
    }, [isOpen])

    const loadBlocks = () => {
        setBlocksLoading(true)
        setBlocksError(null)
        BillingoSettingsService.getBlocks()
            .then(setBlocks)
            .catch((err) => setBlocksError(err.message))
            .finally(() => setBlocksLoading(false))
    }

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
                block_id: blockId.trim(),
                default_unit: defaultUnit.trim() || 'db',
                default_unit_price_type: defaultUnitPriceType
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
                            <div className="flex items-center justify-between mb-1">
                                <label htmlFor="block_id" className="block text-sm font-medium text-foreground">
                                    Invoice Block
                                </label>
                                <button
                                    type="button"
                                    onClick={loadBlocks}
                                    disabled={loading || blocksLoading || !currentMaskedKey}
                                    className="text-xs text-primary hover:underline disabled:opacity-50 disabled:no-underline"
                                >
                                    {blocksLoading ? 'Loading...' : 'Load blocks'}
                                </button>
                            </div>

                            {blocksError && (
                                <div className="text-xs text-destructive mb-1">{blocksError}</div>
                            )}
                            {!currentMaskedKey && (
                                <div className="text-xs text-muted-foreground mb-1">
                                    Save an API key first to load your Billingo invoice blocks
                                </div>
                            )}

                            {blocks.length > 0 ? (
                                <select
                                    id="block_id"
                                    value={blockId}
                                    onChange={(e) => setBlockId(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                >
                                    <option value="">Select an invoice block</option>
                                    {blocks.map((block) => (
                                        <option key={block.id} value={block.id}>
                                            {block.name} ({block.prefix})
                                        </option>
                                    ))}
                                </select>
                            ) : (
                                <input
                                    type="text"
                                    id="block_id"
                                    value={blockId}
                                    onChange={(e) => setBlockId(e.target.value)}
                                    placeholder="Numeric block id"
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                />
                            )}
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label htmlFor="default_unit" className="block text-sm font-medium text-foreground mb-1">
                                    Default Invoice Unit
                                </label>
                                <input
                                    type="text"
                                    id="default_unit"
                                    value={defaultUnit}
                                    onChange={(e) => setDefaultUnit(e.target.value)}
                                    placeholder="db, óra, ..."
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                />
                            </div>
                            <div>
                                <label htmlFor="default_unit_price_type" className="block text-sm font-medium text-foreground mb-1">
                                    Default Price Type
                                </label>
                                <select
                                    id="default_unit_price_type"
                                    value={defaultUnitPriceType}
                                    onChange={(e) => setDefaultUnitPriceType(e.target.value as 'net' | 'gross')}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                >
                                    <option value="net">Net</option>
                                    <option value="gross">Gross</option>
                                </select>
                            </div>
                        </div>
                        <p className="text-xs text-muted-foreground -mt-2">
                            Used for every invoice unless a client has its own override (Clients → Edit Client)
                        </p>

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
