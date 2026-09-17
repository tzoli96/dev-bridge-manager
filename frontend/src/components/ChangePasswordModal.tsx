'use client'

import { useState } from 'react'
import { useAuth } from '@/contexts/AuthContext'
import { UsersService, ChangePasswordRequest } from '@/services/usersService'

interface ChangePasswordModalProps {
    isOpen: boolean
    onClose: () => void
    onSuccess?: () => void
}

export default function ChangePasswordModal({ isOpen, onClose, onSuccess }: ChangePasswordModalProps) {
    const { logout } = useAuth()
    const [formData, setFormData] = useState<ChangePasswordRequest>({
        current_password: '',
        new_password: '',
        confirm_password: ''
    })
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)
    const [passwordStrength, setPasswordStrength] = useState<string>('')

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { name, value } = e.target
        setFormData(prev => ({
            ...prev,
            [name]: value
        }))
        setError(null)

        // Jelszó erősség ellenőrzés
        if (name === 'new_password') {
            checkPasswordStrength(value)
        }
    }

    const checkPasswordStrength = (password: string) => {
        if (password.length === 0) {
            setPasswordStrength('')
            return
        }

        let strength = 0
        const checks = [
            password.length >= 8,
            /[a-z]/.test(password),
            /[A-Z]/.test(password),
            /[0-9]/.test(password),
            /[^A-Za-z0-9]/.test(password)
        ]

        strength = checks.filter(Boolean).length

        if (strength < 2) setPasswordStrength('Weak')
        else if (strength < 4) setPasswordStrength('Medium')
        else setPasswordStrength('Strong')
    }

    const handleSubmit = async () => {
        // Validációk
        if (!formData.current_password) {
            setError('Current password is required')
            return
        }

        if (!formData.new_password) {
            setError('New password is required')
            return
        }

        if (formData.new_password.length < 6) {
            setError('New password must be at least 6 characters long')
            return
        }

        if (formData.new_password !== formData.confirm_password) {
            setError('Password confirmation does not match')
            return
        }

        if (formData.current_password === formData.new_password) {
            setError('New password must be different from current password')
            return
        }

        try {
            setLoading(true)
            setError(null)

            const message = await UsersService.changePassword(formData)
            setSuccess(true)

            // 3 másodperc után automatikus kijelentkezés
            setTimeout(() => {
                logout() // Automatikus kijelentkezés
                onClose()
                setSuccess(false)
                onSuccess?.()
            }, 3000)

        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to change password')
        } finally {
            setLoading(false)
        }
    }

    const handleClose = () => {
        if (!loading && !success) {
            setFormData({
                current_password: '',
                new_password: '',
                confirm_password: ''
            })
            setError(null)
            setPasswordStrength('')
            onClose()
        }
    }

    const getPasswordStrengthColor = () => {
        switch (passwordStrength) {
            case 'Weak': return 'text-destructive'
            case 'Medium': return 'text-warning'
            case 'Strong': return 'text-success'
            default: return 'text-muted-foreground'
        }
    }

    if (!isOpen) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md mx-4">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Change Password</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading || success}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50 text-xl font-bold"
                    >
                        ✕
                    </button>
                </div>

                {success ? (
                    <div className="text-center py-8">
                        <div className="text-success mb-4">
                            <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                        </div>
                        <h3 className="text-lg font-medium text-success mb-2">Password Changed!</h3>
                        <p className="text-success mb-4">Your password has been successfully changed.</p>
                        <p className="text-sm text-muted-foreground">You will be logged out automatically in a few seconds...</p>
                        <div className="mt-4">
                            <div className="animate-spin rounded-full h-6 w-6 border-2 border-success border-t-transparent mx-auto"></div>
                        </div>
                    </div>
                ) : (
                    <div className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-4 py-3 rounded">
                                {error}
                            </div>
                        )}

                        <div>
                            <label htmlFor="current_password" className="block text-sm font-medium text-foreground mb-1">
                                Current Password *
                            </label>
                            <input
                                type="password"
                                id="current_password"
                                name="current_password"
                                value={formData.current_password}
                                onChange={handleChange}
                                required
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                                placeholder="Enter your current password"
                            />
                        </div>

                        <div>
                            <label htmlFor="new_password" className="block text-sm font-medium text-foreground mb-1">
                                New Password *
                            </label>
                            <input
                                type="password"
                                id="new_password"
                                name="new_password"
                                value={formData.new_password}
                                onChange={handleChange}
                                required
                                minLength={6}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                                placeholder="Enter new password (min. 6 characters)"
                            />
                            {passwordStrength && (
                                <p className={`text-xs mt-1 ${getPasswordStrengthColor()}`}>
                                    Password strength: {passwordStrength}
                                </p>
                            )}
                        </div>

                        <div>
                            <label htmlFor="confirm_password" className="block text-sm font-medium text-foreground mb-1">
                                Confirm New Password *
                            </label>
                            <input
                                type="password"
                                id="confirm_password"
                                name="confirm_password"
                                value={formData.confirm_password}
                                onChange={handleChange}
                                required
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                                placeholder="Confirm your new password"
                            />
                            {formData.confirm_password && formData.new_password !== formData.confirm_password && (
                                <p className="text-xs text-destructive mt-1">
                                    Passwords do not match
                                </p>
                            )}
                        </div>

                        <div className="bg-warning/10 border border-warning/20 rounded-lg p-3">
                            <div className="flex">
                                <div className="flex-shrink-0">
                                    <svg className="h-5 w-5 text-warning" viewBox="0 0 20 20" fill="currentColor">
                                        <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
                                    </svg>
                                </div>
                                <div className="ml-3">
                                    <p className="text-sm text-warning">
                                        <strong>Important:</strong> After changing your password, you will be automatically logged out and need to log in again with your new password.
                                    </p>
                                </div>
                            </div>
                        </div>

                        <div className="flex gap-3 pt-4">
                            <button
                                type="button"
                                onClick={handleClose}
                                disabled={loading}
                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSubmit}
                                disabled={loading || !formData.current_password || !formData.new_password || !formData.confirm_password}
                                className="flex-1 px-4 py-2 bg-destructive text-white rounded-lg hover:bg-destructive/90 disabled:opacity-50 transition-colors flex items-center justify-center"
                            >
                                {loading ? (
                                    <>
                                        <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent mr-2"></div>
                                        Changing...
                                    </>
                                ) : (
                                    'Change Password'
                                )}
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}