'use client'

import { useState, useEffect } from 'react'
import { apiClient } from '@/lib/api'

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
        <div className="p-6 max-w-md mx-auto bg-white rounded-xl shadow-lg">
            <h2 className="text-2xl font-bold mb-4 text-gray-800">
                Backend Health Check
            </h2>

            <button
                onClick={checkHealth}
                disabled={loading}
                className={`w-full px-4 py-2 rounded-lg font-semibold transition-colors ${
                    loading
                        ? 'bg-gray-400 cursor-not-allowed'
                        : 'bg-blue-500 hover:bg-blue-600 text-white'
                }`}
            >
                {loading ? '🔄 Checking...' : '🏥 Check Health'}
            </button>

            {/* Results */}
            <div className="mt-4 space-y-3">
                {error && (
                    <div className="p-3 bg-red-100 border border-red-400 text-red-700 rounded">
                        <strong>❌ Error:</strong> {error}
                    </div>
                )}

                {health && (
                    <div className="p-3 bg-green-100 border border-green-400 text-green-700 rounded">
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
                    <div className="text-sm text-gray-500">
                        Last checked: {lastChecked}
                    </div>
                )}
            </div>

            {/* API URL Info */}
            <div className="mt-4 p-2 bg-gray-100 rounded text-xs text-gray-600">
                <strong>API URL:</strong> {process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1'}
            </div>
        </div>
    )
}