'use client'

import { useState } from 'react'
import { useAuth } from '../../contexts/AuthContext'

interface RegisterFormProps {
    onSuccess?: () => void
    onToggleForm?: () => void
}

export default function RegisterForm({ onSuccess, onToggleForm }: RegisterFormProps) {
    const [formData, setFormData] = useState({
        name: '',
        email: '',
        password: '',
        position: ''
    })
    const [error, setError] = useState('')
    const { register, loading } = useAuth()

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData(prev => ({
            ...prev,
            [e.target.name]: e.target.value
        }))
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        try {
            await register(formData)
            onSuccess?.()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Registration failed')
        }
    }

    return (
        <div className="w-full max-w-md mx-auto">
            <div className="bg-white rounded-lg shadow-md p-6">
                <h2 className="text-2xl font-bold text-center mb-6 text-gray-800">
                    Sign Up
                </h2>

                {error && (
                    <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded mb-4">
                        {error}
                    </div>
                )}

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                            Full Name
                        </label>
                        <input
                            type="text"
                            id="name"
                            name="name"
                            value={formData.name}
                            onChange={handleChange}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            placeholder="Enter your full name"
                        />
                    </div>

                    <div>
                        <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
                            Email
                        </label>
                        <input
                            type="email"
                            id="email"
                            name="email"
                            value={formData.email}
                            onChange={handleChange}
                            required
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            placeholder="Enter your email"
                        />
                    </div>

                    <div>
                        <label htmlFor="position" className="block text-sm font-medium text-gray-700 mb-1">
                            Position
                        </label>
                        <input
                            type="text"
                            id="position"
                            name="position"
                            value={formData.position}
                            onChange={handleChange}
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            placeholder="Enter your position (optional)"
                        />
                    </div>

                    <div>
                        <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                            Password
                        </label>
                        <input
                            type="password"
                            id="password"
                            name="password"
                            value={formData.password}
                            onChange={handleChange}
                            required
                            minLength={6}
                            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            placeholder="Enter your password (min 6 characters)"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className={`w-full py-2 px-4 rounded-md font-semibold text-white transition-colors ${
                            loading
                                ? 'bg-gray-400 cursor-not-allowed'
                                : 'bg-green-600 hover:bg-green-700'
                        }`}
                    >
                        {loading ? 'Creating account...' : 'Sign Up'}
                    </button>
                </form>

                {onToggleForm && (
                    <div className="mt-4 text-center">
                        <p className="text-sm text-gray-600">
                            Already have an account?{' '}
                            <button
                                onClick={onToggleForm}
                                className="text-blue-600 hover:text-blue-800 font-medium"
                            >
                                Sign in
                            </button>
                        </p>
                    </div>
                )}
            </div>
        </div>
    )
}