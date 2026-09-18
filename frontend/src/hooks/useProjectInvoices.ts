'use client'

import React from 'react'
import { ProjectsService, Project } from '@/services/projectsService'
import { InvoicesService, Invoice } from '@/services/invoicesService'
import { useAuth } from '@/hooks/auth/use-auth'
import { hasPermission } from '@/utils/permissions'

export function useProjectInvoices(projectId: string) {
    const { user } = useAuth()
    const [project, setProject] = React.useState<Project | null>(null)
    const [invoices, setInvoices] = React.useState<Invoice[]>([])
    const [loading, setLoading] = React.useState(true)

    const reload = React.useCallback(() => {
        return ProjectsService.getProject(Number(projectId)).then((p) => {
            setProject(p)
            if (hasPermission(user, 'invoices.read')) {
                return InvoicesService.getProjectInvoices(p.id).then(setInvoices).catch(() => setInvoices([]))
            }
        }).catch(() => setProject(null))
    }, [projectId, user])

    React.useEffect(() => {
        setLoading(true)
        reload().finally(() => setLoading(false))
    }, [reload])

    return { user, project, invoices, loading, reload }
}
