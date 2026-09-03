import { useState, useEffect, useCallback } from 'react';
import { ProjectsService, Project } from '@/services/projectsService';

interface UseProjectReturn {
    project: Project | null;
    loading: boolean;
    error: string | null;
    refetch: () => Promise<void>;
}

export const useProject = (projectId: string): UseProjectReturn => {
    const [project, setProject] = useState<Project | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const fetchProject = useCallback(async () => {
        if (!projectId) {
            setLoading(false);
            return;
        }

        try {
            setLoading(true);
            setError(null);
            const data = await ProjectsService.getProject(Number(projectId));
            setProject(data);
        } catch (err: any) {
            setError(err.message);
            setProject(null);
        } finally {
            setLoading(false);
        }
    }, [projectId]);

    useEffect(() => {
        fetchProject();
    }, [fetchProject]);

    return { project, loading, error, refetch: fetchProject };
};
