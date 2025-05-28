export interface User {
    id: number
    name: string
    email: string
    position: string
}

export interface AuthResponse {
    success: boolean
    message: string
    token?: string
    user?: User
}

export interface LoginRequest {
    email: string
    password: string
}

export interface RegisterRequest {
    name: string
    email: string
    password: string
    position: string
}

export interface AuthContextType {
    user: User | null
    token: string | null
    login: (email: string, password: string) => Promise<void>
    register: (data: RegisterRequest) => Promise<void>
    logout: () => void
    loading: boolean
    isAuthenticated: boolean
}