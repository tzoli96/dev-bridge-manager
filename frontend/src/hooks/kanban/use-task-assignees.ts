'use client';

import { useState, useCallback } from 'react';
import { ProjectAssignmentService } from '@/services/projectAssignmentService';
import type { ProjectAssignment } from '@/services/projectAssignmentService';

export const useTaskAssignees = (projectId: string) => {
    const [assignees, setAssignees] = useState<ProjectAssignment[]>([]);
    const [isLoading, setIsLoading] = useState(false);

    const loadAssignees = useCallback(async () => {
        setIsLoading(true);
        try {
            const list = await ProjectAssignmentService.getProjectAssignments(Number(projectId));
            setAssignees(list.filter(a => a.is_active));
        } finally {
            setIsLoading(false);
        }
    }, [projectId]);

    return { assignees, isLoading, loadAssignees };
};
