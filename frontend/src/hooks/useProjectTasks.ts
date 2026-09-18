'use client'

import React from 'react'
import { taskService } from '@/services/kanban'
import type { Task } from '@/types/kanban'

export function useProjectTasks(projectId: string) {
    const [tasks, setTasks] = React.useState<Task[]>([])
    const [loading, setLoading] = React.useState(true)

    React.useEffect(() => {
        setLoading(true)
        taskService.getTasks(projectId)
            .then(setTasks)
            .catch(() => setTasks([]))
            .finally(() => setLoading(false))
    }, [projectId])

    return { tasks, loading }
}
