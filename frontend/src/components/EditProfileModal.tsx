'use client'

import { useState } from 'react'
import { useAuth } from '@/contexts/AuthContext'
import { UsersService, UpdateProfileRequest } from '@/services/usersService'

interface EditProfileModalProps {
    isOpen: boolean
    onClose: () => void
    onSuccess?: () => void
}

export default function EditProfileModal({ isOpen, onClose, onSuccess }: EditProfileModalProps) {
    const { user, setUser } = useAuth()
    const [formData, setFormData] = useState<UpdateProfileRequest>({
        name: user?.name || '',
        email: user?.email || '',
        position: user?.position || ''
    })
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { name, value } = e.target
        setFormData(prev => ({
            ...prev,
            [name]: value
        }))
        setError(null)
    }

    const handleSubmit = async (e?: React.FormEvent) => {
        if (e) e.preventDefault()

        // Ellenőrizzük, hogy van-e változás
        const hasChanges =
            formData.name !== user?.name ||
            formData.email !== user?.email ||
            formData.position !== user?.position

        if (!hasChanges) {
            setError('No changes detected')
            return
        }

        // Csak a megváltozott mezőket küldjük
        const updateData: UpdateProfileRequest = {}
        if (formData.name !== user?.name && formData.name?.trim()) {
            updateData.name = formData.name.trim()
        }
        if (formData.email !== user?.email && formData.email?.trim()) {
            updateData.email = formData.email.trim()
        }
        if (formData.position !== user?.position) {
            updateData.position = formData.position?.trim() || ''
        }

        try {
            setLoading(true)
            setError(null)

            const updatedUser = await UsersService.updateProfile(updateData)

            // AuthContext frissítése az új user adatokkal
            if (setUser) {
                setUser(updatedUser)
            }

            setSuccess(true)

            // 2 másodperc után bezárjuk a modalt
            setTimeout(() => {
                onClose()
                setSuccess(false)
                onSuccess?.()
            }, 2000)

        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to update profile')
        } finally {
            setLoading(false)
        }
    }

    const handleClose = () => {
        if (!loading) {
            setFormData({
                name: user?.name || '',
                email: user?.email || '',
                position: user?.position || ''
            })
            setError(null)
            setSuccess(false)
            onClose()
        }
    }

    if (!isOpen) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md mx-4">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Edit Profile</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50 text-xl font-bold"
                    >
                        ✕
                    </button>
                </div>

                {success ? (
                    <div className="text-center py-8">
                        <div className="text-success mb-4">
                            <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                        </div>
                        <h3 className="text-lg font-medium text-success mb-2">Profile Updated!</h3>
                        <p className="text-success">Your profile has been successfully updated.</p>
                    </div>
                ) : (
                    <div className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-4 py-3 rounded">
                                {error}
                            </div>
                        )}

                        <div>
                            <label htmlFor="name" className="block text-sm font-medium text-foreground mb-1">
                                Name *
                            </label>
                            <input
                                type="text"
                                id="name"
                                name="name"
                                value={formData.name}
                                onChange={handleChange}
                                required
                                minLength={2}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="email" className="block text-sm font-medium text-foreground mb-1">
                                Email *
                            </label>
                            <input
                                type="email"
                                id="email"
                                name="email"
                                value={formData.email}
                                onChange={handleChange}
                                required
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="position" className="block text-sm font-medium text-foreground mb-1">
                                Position
                            </label>
                            <input
                                type="text"
                                id="position"
                                name="position"
                                value={formData.position}
                                onChange={handleChange}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
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
                                disabled={loading}
                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors flex items-center justify-center"
                            >
                                {loading ? (
                                    <>
                                        <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent mr-2"></div>
                                        Updating...
                                    </>
                                ) : (
                                    'Update Profile'
                                )}
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}