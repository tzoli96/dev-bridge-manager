'use client'

import { useState, useEffect } from 'react'
import { UsersService, UpdateUserRequest, Role } from '@/services/usersService'
import { User } from '@/types/user'

interface EditUserModalProps {
    isOpen: boolean
    user: User | null
    onClose: () => void
    onSuccess?: (user: any) => void
}

export default function EditUserModal({ isOpen, user, onClose, onSuccess }: EditUserModalProps) {
    const [formData, setFormData] = useState<UpdateUserRequest>({
        name: '',
        email: '',
        position: '',
        role_name: '',
        password: ''
    })
    const [roles, setRoles] = useState<Role[]>([])
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)
    const [passwordStrength, setPasswordStrength] = useState<string>('')
    const [showPasswordField, setShowPasswordField] = useState(false)

    // Form inicializálás amikor user változik
    useEffect(() => {
        if (isOpen && user) {
            setFormData({
                name: user.name || '',
                email: user.email || '',
                position: user.position || '',
                role_name: user.role?.name || '',
                password: ''
            })
            loadRoles()
        }
    }, [isOpen, user])

    const loadRoles = async () => {
        try {
            const rolesData = await UsersService.getRoles()
            setRoles(rolesData)
        } catch (error: any) {
            setError(`Failed to load roles: ${error.message}`)
        }
    }

    const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
        const { name, value } = e.target
        setFormData(prev => ({
            ...prev,
            [name]: value
        }))
        setError(null)

        // Jelszó erősség ellenőrzés
        if (name === 'password') {
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

    const getPasswordStrengthColor = () => {
        switch (passwordStrength) {
            case 'Weak': return 'text-red-600'
            case 'Medium': return 'text-yellow-600'
            case 'Strong': return 'text-green-600'
            default: return 'text-gray-600'
        }
    }

    const handleSubmit = async () => {
        if (!user) return

        // Ellenőrizzük, hogy van-e változás
        const hasChanges =
            formData.name !== user.name ||
            formData.email !== user.email ||
            formData.position !== user.position ||
            formData.role_name !== user.role?.name ||
            (showPasswordField && formData.password !== '')

        if (!hasChanges) {
            setError('No changes detected')
            return
        }

        // Validációk
        if (!formData.name?.trim()) {
            setError('Name is required')
            return
        }

        if (!formData.email?.trim()) {
            setError('Email is required')
            return
        }

        if (showPasswordField && formData.password && formData.password.length < 6) {
            setError('Password must be at least 6 characters long')
            return
        }

        // Csak a megváltozott mezőket küldjük
        const updateData: UpdateUserRequest = {}
        if (formData.name !== user.name) updateData.name = formData.name
        if (formData.email !== user.email) updateData.email = formData.email
        if (formData.position !== user.position) updateData.position = formData.position
        if (formData.role_name !== user.role?.name) updateData.role_name = formData.role_name
        if (showPasswordField && formData.password) updateData.password = formData.password

        try {
            setLoading(true)
            setError(null)

            const updatedUser = await UsersService.updateUser(user.id, updateData)
            setSuccess(true)

            // 2 másodperc után bezárjuk a modalt
            setTimeout(() => {
                onClose()
                setSuccess(false)
                onSuccess?.(updatedUser)
                resetForm()
            }, 2000)

        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to update user')
        } finally {
            setLoading(false)
        }
    }

    const resetForm = () => {
        setFormData({
            name: '',
            email: '',
            position: '',
            role_name: '',
            password: ''
        })
        setError(null)
        setPasswordStrength('')
        setSuccess(false)
        setShowPasswordField(false)
    }

    const handleClose = () => {
        if (!loading && !success) {
            resetForm()
            onClose()
        }
    }

    if (!isOpen || !user) return null

    return (
        <div className="fixed inset-0 bg-black/10 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg shadow-lg p-6 w-full max-w-md mx-4">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Edit User</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading || success}
                        className="text-gray-400 hover:text-gray-600 disabled:opacity-50 text-xl font-bold"
                    >
                        ✕
                    </button>
                </div>

                {success ? (
                    <div className="text-center py-8">
                        <div className="text-green-600 mb-4">
                            <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                        </div>
                        <h3 className="text-lg font-medium text-green-900 mb-2">User Updated!</h3>
                        <p className="text-green-700">The user has been successfully updated.</p>
                    </div>
                ) : (
                    <div className="space-y-4">
                        {error && (
                            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
                                {error}
                            </div>
                        )}

                        <div>
                            <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                                Name *
                            </label>
                            <input
                                type="text"
                                id="name"
                                name="name"
                                value={formData.name}
                                onChange={handleChange}
                                required
                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                disabled={loading}
                                placeholder="Enter full name"
                            />
                        </div>

                        <div>
                            <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
                                Email *
                            </label>
                            <input
                                type="email"
                                id="email"
                                name="email"
                                value={formData.email}
                                onChange={handleChange}
                                required
                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                disabled={loading}
                                placeholder="Enter email address"
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
                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                disabled={loading}
                                placeholder="Enter job position"
                            />
                        </div>

                        <div>
                            <label htmlFor="role_name" className="block text-sm font-medium text-gray-700 mb-1">
                                Role *
                            </label>
                            <select
                                id="role_name"
                                name="role_name"
                                value={formData.role_name}
                                onChange={handleChange}
                                required
                                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                disabled={loading}
                            >
                                <option value="">Select a role</option>
                                {roles.map((role) => (
                                    <option key={role.id} value={role.name}>
                                        {role.display_name}
                                    </option>
                                ))}
                            </select>
                        </div>

                        {/* Password section */}
                        <div>
                            <div className="flex items-center justify-between mb-1">
                                <label className="block text-sm font-medium text-gray-700">
                                    Password
                                </label>
                                <button
                                    type="button"
                                    onClick={() => setShowPasswordField(!showPasswordField)}
                                    className="text-xs text-blue-600 hover:text-blue-800"
                                    disabled={loading}
                                >
                                    {showPasswordField ? 'Hide password field' : 'Change password'}
                                </button>
                            </div>

                            {showPasswordField && (
                                <>
                                    <input
                                        type="password"
                                        id="password"
                                        name="password"
                                        value={formData.password}
                                        onChange={handleChange}
                                        minLength={6}
                                        className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        disabled={loading}
                                        placeholder="Enter new password (min. 6 characters)"
                                    />
                                    {passwordStrength && (
                                        <p className={`text-xs mt-1 ${getPasswordStrengthColor()}`}>
                                            Password strength: {passwordStrength}
                                        </p>
                                    )}
                                    <p className="text-xs text-gray-500 mt-1">
                                        Leave empty to keep current password
                                    </p>
                                </>
                            )}
                        </div>

                        <div className="flex gap-3 pt-4">
                            <button
                                type="button"
                                onClick={handleClose}
                                disabled={loading}
                                className="flex-1 px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 disabled:opacity-50 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSubmit}
                                disabled={loading}
                                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors flex items-center justify-center"
                            >
                                {loading ? (
                                    <>
                                        <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent mr-2"></div>
                                        Updating...
                                    </>
                                ) : (
                                    'Update User'
                                )}
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}