'use client'

import { useState, useEffect } from 'react'
import { apiClient } from '@/lib/api'
import { Button } from '@/components/ui/button'

interface HealthResponse {
    status: string
    message: string
    version: string
    environment?: string
}

export default function HealthCheck() {
    const [health, setHealth] = useState<HealthResponse | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [lastChecked, setLastChecked] = useState<string | null>(null)

    const checkHealth = async () => {
        setLoading(true)
        setError(null)

        try {
            const response = await apiClient.healthCheck() as HealthResponse
            setHealth(response)
            setLastChecked(new Date().toLocaleTimeString())
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Unknown error')
            setHealth(null)
        } finally {
            setLoading(false)
        }
    }

    // Auto check on mount
    useEffect(() => {
        checkHealth()
    }, [])

    return (
        <div className="p-6 max-w-md mx-auto bg-card rounded-xl shadow-sm">
            <h2 className="text-2xl font-bold mb-4 text-foreground">
                Backend Health Check
            </h2>

            <Button onClick={checkHealth} loading={loading} className="w-full">
                {loading ? 'Checking...' : '🏥 Check Health'}
            </Button>

            {/* Results */}
            <div className="mt-4 space-y-3">
                {error && (
                    <div className="p-3 bg-destructive/10 border border-destructive/40 text-destructive rounded">
                        <strong>❌ Error:</strong> {error}
                    </div>
                )}

                {health && (
                    <div className="p-3 bg-success/10 border border-success/40 text-success rounded">
                        <div className="space-y-1">
                            <div><strong>✅ Status:</strong> {health.status}</div>
                            <div><strong>📝 Message:</strong> {health.message}</div>
                            <div><strong>🏷️ Version:</strong> {health.version}</div>
                            {health.environment && (
                                <div><strong>🌍 Environment:</strong> {health.environment}</div>
                            )}
                        </div>
                    </div>
                )}

                {lastChecked && (
                    <div className="text-sm text-muted-foreground">
                        Last checked: {lastChecked}
                    </div>
                )}
            </div>

            {/* API URL Info */}
            <div className="mt-4 p-2 bg-muted rounded text-xs text-muted-foreground">
                <strong>API URL:</strong> {process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1'}
            </div>
        </div>
    )
}