'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/contexts/AuthContext'
import { usePermissions } from '@/hooks/usePermissions'


interface ProtectedRouteProps {
    children: React.ReactNode;
    permission?: string;
    permissions?: string[];
    role?: string;
    requireAll?: boolean;
    redirectTo?: string;
    fallback?: React.ReactNode;
}

export function ProtectedRoute({
                                   children,
                                   permission,
                                   permissions,
                                   role,
                                   requireAll = false,
                                   redirectTo = '/auth',
                                   fallback
                               }: ProtectedRouteProps) {
    const { user, loading, isAuthenticated } = useAuth();
    const { canAccess, hasRole } = usePermissions();
    const router = useRouter();

    useEffect(() => {
        // Ha nincs bejelentkezve, irányítsuk át
        if (!loading && !isAuthenticated) {
            router.push(redirectTo);
            return;
        }

        // Ha be van jelentkezve, de nincs jogosultsága
        if (isAuthenticated && user) {
            let hasAccess = true;

            if (role && !hasRole(role)) {
                hasAccess = false;
            }

            if (permission) {
                hasAccess = canAccess(permission);
            }

            if (permissions && permissions.length > 0) {
                hasAccess = canAccess(permissions);
            }

            if (!hasAccess) {
                router.push('/unauthorized'); // Vagy '/dashboard'
                return;
            }
        }
    }, [loading, isAuthenticated, user, permission, permissions, role]);

    // Loading state
    if (loading) {
        return (
            <div className="min-h-screen bg-gray-50 flex items-center justify-center">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                <span className="ml-2 text-gray-600">Loading...</span>
            </div>
        );
    }

    // Ha nincs bejelentkezve
    if (!isAuthenticated) {
        return fallback || null;
    }

    return <>{children}</>;
}