'use client'

import React, { createContext, useContext, useEffect, useState } from 'react'
import { AuthContextType, User, RegisterRequest } from '../types/auth'
import { AuthService } from '../services/authService'
import { apiClient } from '../lib/api'

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null)
    const [token, setToken] = useState<string | null>(null)
    const [loading, setLoading] = useState(true)

    // Initialize auth state from localStorage
    useEffect(() => {
        const initAuth = async () => {
            try {
                const savedToken = AuthService.getToken()
                const savedUser = AuthService.getUser()

                if (savedToken && savedUser) {
                    setToken(savedToken)
                    setUser(savedUser)
                    apiClient.setAuthToken(savedToken)
                }
            } catch (error) {
                console.error('Auth initialization failed:', error)
                // Clear invalid auth data
                AuthService.removeToken()
                AuthService.removeUser()
            } finally {
                setLoading(false)
            }
        }

        initAuth()
    }, [])

    const login = async (email: string, password: string): Promise<void> => {
        try {
            setLoading(true)
            const response = await AuthService.login({ email, password })

            if (response.success && response.token && response.user) {
                // Save auth data
                AuthService.saveToken(response.token)
                AuthService.saveUser(response.user)

                // Update state
                setToken(response.token)
                setUser(response.user)

                // Set auth header for future requests
                apiClient.setAuthToken(response.token)
            } else {
                throw new Error(response.message || 'Login failed')
            }
        } catch (error) {
            console.error('Login error:', error)
            throw error
        } finally {
            setLoading(false)
        }
    }

    const register = async (userData: RegisterRequest): Promise<void> => {
        try {
            setLoading(true)
            const response = await AuthService.register(userData)

            if (response.success && response.token && response.user) {
                // Save auth data
                AuthService.saveToken(response.token)
                AuthService.saveUser(response.user)

                // Update state
                setToken(response.token)
                setUser(response.user)

                // Set auth header for future requests
                apiClient.setAuthToken(response.token)
            } else {
                throw new Error(response.message || 'Registration failed')
            }
        } catch (error) {
            console.error('Registration error:', error)
            throw error
        } finally {
            setLoading(false)
        }
    }

    const logout = (): void => {
        // Clear auth data
        AuthService.removeToken()
        AuthService.removeUser()

        // Clear state
        setToken(null)
        setUser(null)

        // Remove auth header
        apiClient.removeAuthToken()
    }

    const value: AuthContextType = {
        user,
        token,
        login,
        register,
        logout,
        loading,
        isAuthenticated: !!token && !!user
    }

    return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextType {
    const context = useContext(AuthContext)
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider')
    }
    return context
}