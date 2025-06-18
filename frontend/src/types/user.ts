// user.ts - Backend response-szal kompatibilis

export interface User {
    id: number
    name: string
    email: string
    position: string
    role_id?: number  // Backend field
    created_at?: string
    updated_at?: string
    role?: {
        id: number
        name: string
        display_name: string
        description?: string
        is_active?: boolean
        created_at?: string
        updated_at?: string
    }
    permissions?: string[]
}

export interface UsersResponse {
    success: boolean
    data: User[]
    count: number
}

// Backend kompatibilis UserListResponse
export interface UserListResponse {
    success: boolean
    message: string
    users?: User[]
    user?: User
}

export interface ApiError {
    error: boolean
    message: string
}