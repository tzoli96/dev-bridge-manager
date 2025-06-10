import { useAuth } from '../contexts/AuthContext';
import { User } from '../types/auth';

export interface UsePermissionsReturn {
    hasPermission: (permission: string) => boolean;
    hasAnyPermission: (permissions: string[]) => boolean;
    hasRole: (role: string) => boolean;
    canAccess: (requiredPermissions: string | string[]) => boolean;
    user: User | null;
    permissions: string[];
    role: string | null;
}

export function usePermissions(): UsePermissionsReturn {
    const { user, hasPermission, hasAnyPermission, hasRole, canAccess } = useAuth();

    return {
        hasPermission,
        hasAnyPermission,
        hasRole,
        canAccess,
        user,
        permissions: user?.permissions || [],
        role: user?.role?.name || null,
    };
}