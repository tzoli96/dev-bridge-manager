// Dependency-free on purpose (type-only imports): the grouping rule can be
// checked with plain Node, without the Next.js toolchain.
import type { Project, ProjectClient } from '@/services/projectsService'

export const MULTIPLE_LABEL = 'Több ügyfél'
export const NONE_LABEL = 'Ügyfél nélkül'

export type ProjectGroupKind = 'client' | 'multiple' | 'none'

export interface ProjectGroup {
    /** Stable key: client:<id>, multiple or none. Used for the persisted open/closed state. */
    key: string
    kind: ProjectGroupKind
    clientId: number | null
    label: string
    projects: Project[]
}

const collator = new Intl.Collator('hu')

// A project may carry duplicate link rows for the same client: count each client once.
function distinctClients(project: Project): ProjectClient[] {
    const byId = new Map<number, ProjectClient>()
    for (const link of project.clients ?? []) {
        if (!byId.has(link.client_id)) byId.set(link.client_id, link)
    }
    return [...byId.values()]
}

/** The client names of a project, alphabetical, comma separated ("" when none). */
export function projectClientNames(project: Project): string {
    return distinctClients(project)
        .map((link) => link.client_name)
        .sort((a, b) => collator.compare(a, b))
        .join(', ')
}

/**
 * Groups projects by client. One client -> that client's group; two or more
 * -> a shared "Több ügyfél" group; none -> "Ügyfél nélkül". Every project is
 * in exactly one group. Client groups are alphabetical (ties by id), then the
 * shared group, then the one without clients. Projects keep their input order.
 */
export function groupProjectsByClient(projects: Project[]): ProjectGroup[] {
    const byClient = new Map<number, ProjectGroup>()
    const multiple: Project[] = []
    const none: Project[] = []

    for (const project of projects) {
        const clients = distinctClients(project)
        if (clients.length === 0) {
            none.push(project)
        } else if (clients.length > 1) {
            multiple.push(project)
        } else {
            const only = clients[0]
            let group = byClient.get(only.client_id)
            if (!group) {
                group = {
                    key: `client:${only.client_id}`,
                    kind: 'client',
                    clientId: only.client_id,
                    label: only.client_name,
                    projects: [],
                }
                byClient.set(only.client_id, group)
            }
            group.projects.push(project)
        }
    }

    const groups = [...byClient.values()].sort(
        (a, b) => collator.compare(a.label, b.label) || (a.clientId ?? 0) - (b.clientId ?? 0)
    )
    if (multiple.length > 0) {
        groups.push({ key: 'multiple', kind: 'multiple', clientId: null, label: MULTIPLE_LABEL, projects: multiple })
    }
    if (none.length > 0) {
        groups.push({ key: 'none', kind: 'none', clientId: null, label: NONE_LABEL, projects: none })
    }
    return groups
}
