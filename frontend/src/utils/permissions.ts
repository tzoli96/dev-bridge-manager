import { User } from '@/types/user'

export const hasPermission = (user: User | null, permission: string): boolean => {
    return user?.permissions?.includes(permission) || false
}

export const hasAnyPermission = (user: User | null, permissions: string[]): boolean => {
    return permissions.some(permission => hasPermission(user, permission))
}

export const isAdmin = (user: User | null): boolean => {
    return user?.role?.name === 'admin' || user?.role?.name === 'super_admin'
}