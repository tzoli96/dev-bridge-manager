'use client'

import { useState } from 'react'
import { User, Mail, Lock, Briefcase, AlertCircle, ArrowRight, Loader2 } from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'

interface RegisterFormProps {
    onSuccess?: () => void
    onToggleForm?: () => void
}

export default function RegisterForm({ onSuccess, onToggleForm }: RegisterFormProps) {
    const [name, setName] = useState('')
    const [email, setEmail] = useState('')
    const [password, setPassword] = useState('')
    const [position, setPosition] = useState('')
    const [error, setError] = useState('')
    const { register, loading } = useAuth()

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        try {
            await register({ name, email, password, position })
            onSuccess?.()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Registration failed')
        }
    }

    return (
        <div className="w-full max-w-md mx-auto">
            <div className="bg-card rounded-xl shadow-sm border border-border p-8">
                <h2 className="text-2xl font-semibold text-center mb-1 text-foreground">
                    Create your account
                </h2>
                <p className="text-center text-sm text-muted-foreground mb-6">
                    Join your team on Dev Bridge Manager
                </p>

                {error && (
                    <div className="flex items-start gap-2 bg-destructive/10 border border-destructive/20 text-destructive px-4 py-3 rounded-lg mb-5 text-sm">
                        <AlertCircle size={16} className="mt-0.5 shrink-0" />
                        <span>{error}</span>
                    </div>
                )}

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label htmlFor="name" className="block text-sm font-medium text-foreground mb-1.5">
                            Name
                        </label>
                        <div className="relative">
                            <User size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                            <input
                                type="text"
                                id="name"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                required
                                className="w-full pl-10 pr-3 py-2.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent transition-shadow"
                                placeholder="Jane Doe"
                            />
                        </div>
                    </div>

                    <div>
                        <label htmlFor="email" className="block text-sm font-medium text-foreground mb-1.5">
                            Email
                        </label>
                        <div className="relative">
                            <Mail size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                            <input
                                type="email"
                                id="email"
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                required
                                className="w-full pl-10 pr-3 py-2.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent transition-shadow"
                                placeholder="you@example.com"
                            />
                        </div>
                    </div>

                    <div>
                        <label htmlFor="password" className="block text-sm font-medium text-foreground mb-1.5">
                            Password
                        </label>
                        <div className="relative">
                            <Lock size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                            <input
                                type="password"
                                id="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                required
                                className="w-full pl-10 pr-3 py-2.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent transition-shadow"
                                placeholder="Create a password"
                            />
                        </div>
                    </div>

                    <div>
                        <label htmlFor="position" className="block text-sm font-medium text-foreground mb-1.5">
                            Position <span className="text-muted-foreground font-normal">(optional)</span>
                        </label>
                        <div className="relative">
                            <Briefcase size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                            <input
                                type="text"
                                id="position"
                                value={position}
                                onChange={(e) => setPosition(e.target.value)}
                                className="w-full pl-10 pr-3 py-2.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent transition-shadow"
                                placeholder="Frontend Developer"
                            />
                        </div>
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className={`w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-lg font-semibold text-white text-sm shadow-sm transition-all ${
                            loading
                                ? 'bg-muted-foreground/20 cursor-not-allowed'
                                : 'bg-primary hover:bg-primary/90 hover:shadow-sm active:scale-[0.98]'
                        }`}
                    >
                        {loading ? (
                            <>
                                <Loader2 size={16} className="animate-spin" />
                                Creating account...
                            </>
                        ) : (
                            <>
                                Create Account
                                <ArrowRight size={16} />
                            </>
                        )}
                    </button>
                </form>

                {onToggleForm && (
                    <p className="mt-6 text-center text-sm text-muted-foreground">
                        Already have an account?{' '}
                        <button
                            type="button"
                            onClick={onToggleForm}
                            className="text-primary hover:text-primary font-medium"
                        >
                            Sign in
                        </button>
                    </p>
                )}
            </div>
        </div>
    )
}
