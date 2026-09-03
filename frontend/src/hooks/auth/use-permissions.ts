import { useAuth } from '@/contexts/AuthContext';

interface UsePermissionsReturn {
    hasPermission: (permission: string, projectId?: string) => boolean;
    hasAnyPermission: (permissions: string[]) => boolean;
}

// Kanban feature code calls hasPermission with colon-syntax names
// (e.g. 'tasks:edit'), but the DB's permission table uses dot-syntax
// (e.g. 'tasks.update'). This map bridges the two; anything not listed
// falls back to a straight ':' -> '.' translation.
const KANBAN_PERMISSION_MAP: Record<string, string> = {
    'tasks:edit': 'tasks.update',
};

const toDotPermission = (permission: string): string =>
    KANBAN_PERMISSION_MAP[permission] ?? permission.replace(':', '.');

// Adapter so kanban feature code can import `@/hooks/auth/use-permissions`.
// The kanban feature calls hasPermission(permission, projectId), but the
// project's permission model isn't project-scoped, so projectId is ignored.
export const usePermissions = (): UsePermissionsReturn => {
    const { hasPermission, hasAnyPermission } = useAuth();

    return {
        hasPermission: (permission: string, _projectId?: string) => hasPermission(toDotPermission(permission)),
        hasAnyPermission,
    };
};
