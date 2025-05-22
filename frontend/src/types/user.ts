export interface User {
    id: number
    name: string
    email: string
    position: string
    created_at: string
    updated_at: string
}

export interface UsersResponse {
    success: boolean
    data: User[]
    count: number
}

export interface ApiError {
    error: boolean
    message: string
}