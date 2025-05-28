'use client'

import { useState } from 'react'
import { useAuth } from '../../contexts/AuthContext'

interface LoginFormProps {
    onSuccess?: () => void
    onToggleForm?: () => void
}

export default function LoginForm({ onSuccess, onToggleForm }: LoginFormProps) {
    const [email, setEmail] = useState('')
    const [password, setPassword] = useState('')
    const [error, setError] = useState('')
    const { login, loading } = useAuth()

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        try {
            await login(email, password)
            onSuccess?.()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Login failed')
        }
    }

    return (
        <div className="w-full max-w-md mx-auto">
            <div className="bg-white rounded-lg shadow-md p-6">
                <h2 className="text-2xl font-bold text-center mb-6 text-gray-800">
                    Sign In
                </h2>

                {error && (
                    <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded mb-4">
                        {error}
                    </div>
                )}

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
                            Email
                        </label>
                        <input
                            type="email"
                            id="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            placeholder="Enter your email"
                        />
                    </div>

                    <div>
                        <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                            Password
                        </label>
                        <input
                            type="password"
                            id="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            placeholder="Enter your password"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className={`w-full py-2 px-4 rounded-md font-semibold text-white transition-colors ${
                            loading
                                ? 'bg-gray-400 cursor-not-allowed'
                                : 'bg-blue-600 hover:bg-blue-700'
                        }`}
                    >
                        {loading ? 'Signing in...' : 'Sign In'}
                    </button>
                </form>

                {onToggleForm && (
                    <div className="mt-4 text-center">
                        <p className="text-sm text-gray-600">
                            Don't have an account?{' '}
                            <button
                                onClick={onToggleForm}
                                className="text-blue-600 hover:text-blue-800 font-medium"
                            >
                                Sign up
                            </button>
                        </p>
                    </div>
                )}
            </div>
        </div>
    )
}