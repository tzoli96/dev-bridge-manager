import { apiClient } from '../lib/api'
import { AuthResponse, LoginRequest, RegisterRequest, User } from '../types/auth'

export class AuthService {
    private static readonly AUTH_ENDPOINT = '/auth'

    static async login(credentials: LoginRequest): Promise<AuthResponse> {
        try {
            const response = await apiClient.post<AuthResponse>(
                `${this.AUTH_ENDPOINT}/login`,
                credentials
            )

            if (!response.success) {
                throw new Error(response.message || 'Login failed')
            }

            return response
        } catch (error: any) {
            console.error('Login failed:', error)

            // Handle different error types
            if (error.response?.status === 401) {
                throw new Error('Invalid email or password')
            } else if (error.response?.status === 400) {
                throw new Error('Please check your email and password')
            } else if (error.message) {
                throw new Error(error.message)
            } else {
                throw new Error('Login failed. Please try again.')
            }
        }
    }

    static async register(userData: RegisterRequest): Promise<AuthResponse> {
        try {
            const response = await apiClient.post<AuthResponse>(
                `${this.AUTH_ENDPOINT}/register`,
                userData
            )

            if (!response.success) {
                throw new Error(response.message || 'Registration failed')
            }

            return response
        } catch (error: any) {
            console.error('Registration failed:', error)

            // Handle different error types
            if (error.response?.status === 409) {
                throw new Error('An account with this email already exists')
            } else if (error.response?.status === 400) {
                throw new Error('Please check your information and try again')
            } else if (error.response?.data?.message) {
                throw new Error(error.response.data.message)
            } else if (error.message) {
                throw new Error(error.message)
            } else {
                throw new Error('Registration failed. Please try again.')
            }
        }
    }

    static async getCurrentUser(token: string): Promise<User> {
        try {
            // Set token for this request
            apiClient.setAuthToken(token)

            const response = await apiClient.get<{ success: boolean; user: User }>(
                `${this.AUTH_ENDPOINT}/me`
            )

            return response.user
        } catch (error) {
            console.error('Failed to get current user:', error)
            throw new Error('Failed to get user info')
        }
    }

    // Token management
    static saveToken(token: string): void {
        if (typeof window !== 'undefined') {
            localStorage.setItem('auth_token', token)
        }
    }

    static getToken(): string | null {
        if (typeof window !== 'undefined') {
            return localStorage.getItem('auth_token')
        }
        return null
    }

    static removeToken(): void {
        if (typeof window !== 'undefined') {
            localStorage.removeItem('auth_token')
        }
    }

    static saveUser(user: User): void {
        if (typeof window !== 'undefined') {
            localStorage.setItem('auth_user', JSON.stringify(user))
        }
    }

    static getUser(): User | null {
        if (typeof window !== 'undefined') {
            const userStr = localStorage.getItem('auth_user')
            return userStr ? JSON.parse(userStr) : null
        }
        return null
    }

    static removeUser(): void {
        if (typeof window !== 'undefined') {
            localStorage.removeItem('auth_user')
        }
    }
}