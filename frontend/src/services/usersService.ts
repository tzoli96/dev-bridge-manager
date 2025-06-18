import { apiClient } from '@/lib/api'
import { User, UsersResponse } from '@/types/user'

export interface UpdateProfileRequest {
    name?: string
    email?: string
    position?: string
}

export interface UpdateProfileResponse {
    success: boolean
    message: string
    user?: User
}

export interface ChangePasswordRequest {
    current_password: string
    new_password: string
    confirm_password: string
}

export interface ChangePasswordResponse {
    success: boolean
    message: string
}

// ÚJ: Admin CRUD interface-ek
export interface CreateUserRequest {
    name: string
    email: string
    password: string
    position?: string
    role_name: string
}

export interface UpdateUserRequest {
    name?: string
    email?: string
    position?: string
    role_name?: string
    password?: string // ÚJ: opcionális jelszó frissítés
}

export interface Role {
    id: number
    name: string
    display_name: string
    description?: string
    is_active: boolean
}

// Backend kompatibilis UserListResponse
export interface UserListResponse {
    success: boolean
    message: string
    users?: User[]
    user?: User
}

export class UsersService {
    private static readonly ENDPOINT = '/users'

    // JAVÍTOTT: Használjuk a public endpoint-ot, ami működik
    static async getAllUsers(): Promise<User[]> {
        try {
            console.log('🔍 [UsersService] Fetching users from /users endpoint')

            // A public endpoint működik és adatokat ad vissza
            const response = await apiClient.get<{ success: boolean; data: User[]; count: number }>('/users')

            console.log('🔍 [UsersService] Response:', response)

            if (response.success && response.data) {
                // A role objektumok üresek lehetnek, javítsuk ki
                const users = response.data.map(user => ({
                    ...user,
                    role: user.role && user.role.id !== 0 ? user.role : {
                        id: user.role_id || 0,
                        name: 'unknown',
                        display_name: 'Unknown Role'
                    }
                }))

                console.log('🔍 [UsersService] Processed users:', users)
                return users
            }

            throw new Error('Invalid response from server')
        } catch (error: any) {
            console.error('Failed to fetch users:', error)
            throw new Error(error.message || 'Failed to load users')
        }
    }

    // ÚJ: Public user listázás (ha szükséges lenne)
    static async getPublicUsers(): Promise<User[]> {
        try {
            // Ez a public endpoint (middleware előtt)
            const response = await apiClient.get<{ success: boolean; data: User[]; count: number }>('/users')
            return response.data || []
        } catch (error) {
            console.error('Failed to fetch public users:', error)
            throw new Error('Failed to load users')
        }
    }

    static async getUserById(id: number): Promise<User> {
        try {
            const response = await apiClient.get<UserListResponse>(`${this.ENDPOINT}/${id}`)

            if (!response.success || !response.user) {
                throw new Error(response.message || 'User not found')
            }

            return response.user
        } catch (error: any) {
            console.error(`Failed to fetch user ${id}:`, error)
            throw new Error(error.message || `Failed to load user with ID ${id}`)
        }
    }

    static async updateProfile(data: UpdateProfileRequest): Promise<User> {
        try {
            const response = await apiClient.put<UpdateProfileResponse>(`${this.ENDPOINT}/profile/me`, data)

            if (!response.success || !response.user) {
                throw new Error(response.message || 'Failed to update profile')
            }

            return response.user
        } catch (error: any) {
            console.error('Failed to update profile:', error)
            throw new Error(error.message || 'Failed to update profile')
        }
    }

    static async changePassword(data: ChangePasswordRequest): Promise<string> {
        try {
            const response = await apiClient.put<ChangePasswordResponse>(`${this.ENDPOINT}/profile/password`, data)

            if (!response.success) {
                throw new Error(response.message || 'Failed to change password')
            }

            return response.message
        } catch (error: any) {
            console.error('Failed to change password:', error)
            throw new Error(error.message || 'Failed to change password')
        }
    }

    // ÚJ: Admin user létrehozás
    static async createUser(userData: CreateUserRequest): Promise<User> {
        try {
            const response = await apiClient.post<UserListResponse>('/users/', userData)

            if (!response.success || !response.user) {
                throw new Error(response.message || 'Failed to create user')
            }

            return response.user
        } catch (error: any) {
            console.error('Failed to create user:', error)
            throw new Error(error.message || 'Failed to create user')
        }
    }

    // ÚJ: Admin user frissítés (jelszóval együtt)
    static async updateUser(id: number, userData: UpdateUserRequest): Promise<User> {
        try {
            const response = await apiClient.put<UserListResponse>(`/users/${id}`, userData)

            if (!response.success || !response.user) {
                throw new Error(response.message || 'Failed to update user')
            }

            return response.user
        } catch (error: any) {
            console.error('Failed to update user:', error)
            throw new Error(error.message || 'Failed to update user')
        }
    }

    // JAVÍTOTT: User törlés
    static async deleteUser(id: number): Promise<void> {
        try {
            const response = await apiClient.delete<UserListResponse>(`/users/${id}`)

            if (!response.success) {
                throw new Error(response.message || 'Failed to delete user')
            }
        } catch (error: any) {
            console.error('Failed to delete user:', error)
            throw new Error(error.message || 'Failed to delete user')
        }
    }

    // JAVÍTOTT: Role-ok lekérése a helyes endpoint-ról
    static async getRoles(): Promise<Role[]> {
        try {
            const response = await apiClient.get<{ success: boolean; roles: Role[]; message: string }>('/roles')

            if (!response.success) {
                throw new Error(response.message || 'Failed to load roles')
            }

            return response.roles || []
        } catch (error: any) {
            console.error('Failed to fetch roles:', error)
            throw new Error(error.message || 'Failed to load roles')
        }
    }
}