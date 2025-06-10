'use client'

import { usePermissions } from '@/hooks/usePermissions'

interface PermissionGuardProps {
    children: React.ReactNode;
    permission?: string;
    permissions?: string[];
    role?: string;
    requireAll?: boolean; // true = minden permission kell, false = bármelyik elég
    fallback?: React.ReactNode;
    showFallback?: boolean;
}

export function PermissionGuard({
                                    children,
                                    permission,
                                    permissions,
                                    role,
                                    requireAll = false,
                                    fallback = null,
                                    showFallback = false
                                }: PermissionGuardProps) {
    const { hasPermission, hasAnyPermission, hasRole, user } = usePermissions();

    // Ha nincs user, ne mutassunk semmit
    if (!user) {
        return showFallback ? <>{fallback}</> : null;
    }

    let hasAccess = true;

    // Role ellenőrzés
    if (role && !hasRole(role)) {
        hasAccess = false;
    }

    // Egyedi permission ellenőrzés
    if (permission && !hasPermission(permission)) {
        hasAccess = false;
    }

    // Több permission ellenőrzés
    if (permissions && permissions.length > 0) {
        if (requireAll) {
            // Minden permission kell
            hasAccess = permissions.every(p => hasPermission(p));
        } else {
            // Bármelyik permission elég
            hasAccess = hasAnyPermission(permissions);
        }
    }

    if (!hasAccess) {
        return showFallback ? <>{fallback}</> : null;
    }

    return <>{children}</>;
}
