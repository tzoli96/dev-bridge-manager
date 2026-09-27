// frontend/src/components/emails/CreateTaskFromEmailModal.tsx
'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Modal } from '@/components/ui/modal'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { useProjects } from '@/hooks/useProjects'
import { boardService, kanbanService, taskService, subtaskService } from '@/services/kanban'
import type { Board, KanbanColumn } from '@/types/kanban'
import { TaskPriority } from '@/types/kanban'
import { EmailsService, type EmailDetail, type TaskBreakdownGroup } from '@/services/emailsService'
import { Plus, Trash2 } from 'lucide-react'

const DESCRIPTION_EXCERPT_LIMIT = 500

// Email bodies come from third-party senders and are never trusted content -
// escape them before they end up in a task's htmlDescription, which
// RichTextEditor injects as raw innerHTML with no sanitization of its own.
function escapeHtml(raw: string): string {
    return raw
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;')
}

function toHtmlDescription(description: string): string {
    return escapeHtml(description).replace(/\n/g, '<br>')
}

function fallbackGroup(email: EmailDetail): EditableGroup {
    const title = email.subject || '(nincs tárgy)'
    const bodySource = email.body_text || ''
    const excerpt = bodySource.length > DESCRIPTION_EXCERPT_LIMIT
        ? bodySource.slice(0, DESCRIPTION_EXCERPT_LIMIT) + '…'
        : bodySource
    const description = `Feladó: ${email.from || 'ismeretlen'}\n\n${excerpt}`
    return { id: newId(), title, description, subtasks: [] }
}

let idCounter = 0
function newId(): string {
    idCounter += 1
    return `draft-${idCounter}`
}

interface EditableItem {
    id: string
    title: string
    description: string
}

interface EditableGroup extends EditableItem {
    subtasks: EditableItem[]
}

function groupsFromBreakdown(groups: TaskBreakdownGroup[]): EditableGroup[] {
    return groups.map((g) => ({
        id: newId(),
        title: g.title,
        description: g.description,
        subtasks: g.subtasks.map((s) => ({ id: newId(), title: s.title, description: s.description })),
    }))
}

interface CreateTaskFromEmailModalProps {
    isOpen: boolean
    onClose: () => void
    email: EmailDetail
}

export function CreateTaskFromEmailModal({ isOpen, onClose, email }: CreateTaskFromEmailModalProps) {
    const router = useRouter()
    const { projects, loading: projectsLoading } = useProjects()

    const [selectedProjectId, setSelectedProjectId] = useState('')
    const [boards, setBoards] = useState<Board[]>([])
    const [boardsLoading, setBoardsLoading] = useState(false)
    const [selectedBoardId, setSelectedBoardId] = useState('')
    const [columns, setColumns] = useState<KanbanColumn[]>([])
    const [columnsLoading, setColumnsLoading] = useState(false)
    const [selectedColumnId, setSelectedColumnId] = useState('')

    const [groups, setGroups] = useState<EditableGroup[]>([])
    const [breakdownLoading, setBreakdownLoading] = useState(false)
    const [breakdownWarning, setBreakdownWarning] = useState<string | null>(null)

    const [creating, setCreating] = useState(false)
    const [createError, setCreateError] = useState<string | null>(null)

    // Always start empty - per design, there's no "last used" default.
    useEffect(() => {
        if (!isOpen) return
        setSelectedProjectId('')
        setBoards([])
        setSelectedBoardId('')
        setColumns([])
        setSelectedColumnId('')
        setCreateError(null)
        setBreakdownWarning(null)

        if (!email.id) {
            setGroups([fallbackGroup(email)])
            setBreakdownLoading(false)
            return
        }

        setBreakdownLoading(true)
        EmailsService.breakdownIntoTasks(email.id)
            .then((res) => {
                if (res.success && res.groups && res.groups.length > 0) {
                    setGroups(groupsFromBreakdown(res.groups))
                } else {
                    setGroups([fallbackGroup(email)])
                    setBreakdownWarning('Az AI nem tudott feladatokat javasolni, egy alap feladatot készítettünk az e-mail alapján - szerkeszd igény szerint.')
                }
            })
            .catch(() => {
                setGroups([fallbackGroup(email)])
                setBreakdownWarning('Az AI-alapú feladat-javaslat nem sikerült, egy alap feladatot készítettünk az e-mail alapján - szerkeszd igény szerint.')
            })
            .finally(() => setBreakdownLoading(false))
    }, [isOpen, email])

    // The backend already computed this at sync time (AI client match plus
    // a deterministic client-has-exactly-one-project check - see
    // services.matchClientAndProject) - just preselect it here, never
    // overriding a project the user already picked.
    useEffect(() => {
        if (!isOpen || !email.project_id) return
        const projectId = String(email.project_id)
        setSelectedProjectId((current) => (current === '' ? projectId : current))
    }, [isOpen, email])

    useEffect(() => {
        if (!selectedProjectId) {
            setBoards([])
            setSelectedBoardId('')
            return
        }
        setBoardsLoading(true)
        setSelectedBoardId('')
        setColumns([])
        setSelectedColumnId('')
        boardService.listBoards(selectedProjectId)
            .then(setBoards)
            .catch(() => setBoards([]))
            .finally(() => setBoardsLoading(false))
    }, [selectedProjectId])

    useEffect(() => {
        if (!selectedProjectId || !selectedBoardId) {
            setColumns([])
            setSelectedColumnId('')
            return
        }
        setColumnsLoading(true)
        setSelectedColumnId('')
        kanbanService.getBoard(selectedProjectId, selectedBoardId)
            .then((board) => setColumns(board.columns))
            .catch(() => setColumns([]))
            .finally(() => setColumnsLoading(false))
    }, [selectedProjectId, selectedBoardId])

    const updateGroup = (groupId: string, patch: Partial<EditableItem>) => {
        setGroups((prev) => prev.map((g) => (g.id === groupId ? { ...g, ...patch } : g)))
    }

    const deleteGroup = (groupId: string) => {
        setGroups((prev) => prev.filter((g) => g.id !== groupId))
    }

    const addGroup = () => {
        setGroups((prev) => [...prev, { id: newId(), title: '', description: '', subtasks: [] }])
    }

    const updateSubtask = (groupId: string, subtaskId: string, patch: Partial<EditableItem>) => {
        setGroups((prev) => prev.map((g) => (
            g.id === groupId
                ? { ...g, subtasks: g.subtasks.map((s) => (s.id === subtaskId ? { ...s, ...patch } : s)) }
                : g
        )))
    }

    const deleteSubtask = (groupId: string, subtaskId: string) => {
        setGroups((prev) => prev.map((g) => (
            g.id === groupId ? { ...g, subtasks: g.subtasks.filter((s) => s.id !== subtaskId) } : g
        )))
    }

    const addSubtask = (groupId: string) => {
        setGroups((prev) => prev.map((g) => (
            g.id === groupId ? { ...g, subtasks: [...g.subtasks, { id: newId(), title: '', description: '' }] } : g
        )))
    }

    const validGroups = groups
        .map((g) => ({ ...g, title: g.title.trim(), subtasks: g.subtasks.filter((s) => s.title.trim()) }))
        .filter((g) => g.title)

    const canConfirm = !!selectedProjectId && !!selectedBoardId && !!selectedColumnId
        && validGroups.length > 0 && !creating && !breakdownLoading

    const handleConfirm = async () => {
        if (!canConfirm) return
        setCreating(true)
        setCreateError(null)
        try {
            for (const group of validGroups) {
                const parent = await taskService.createTask(selectedProjectId, {
                    title: group.title,
                    description: group.description,
                    htmlDescription: toHtmlDescription(group.description),
                    priority: TaskPriority.MEDIUM,
                    columnId: selectedColumnId,
                    boardId: selectedBoardId,
                })
                for (const sub of group.subtasks) {
                    await subtaskService.createSubtask(selectedProjectId, parent.id, {
                        title: sub.title.trim(),
                        description: sub.description,
                        htmlDescription: toHtmlDescription(sub.description),
                        priority: TaskPriority.MEDIUM,
                        columnId: selectedColumnId,
                        boardId: selectedBoardId,
                    })
                }
            }
            onClose()
            router.push(`/dashboard/board/${selectedProjectId}/${selectedBoardId}`)
        } catch (err) {
            setCreateError(err instanceof Error ? err.message : 'Hiba történt a feladatok létrehozása közben.')
        } finally {
            setCreating(false)
        }
    }

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Feladat létrehozása e-mailből" size="lg">
            <div className="p-6 pt-0 space-y-4">
                {breakdownLoading && (
                    <div className="text-sm text-muted-foreground py-2">AI javaslat készítése...</div>
                )}
                {breakdownWarning && (
                    <div className="bg-amber-500/10 border border-amber-500/20 text-amber-700 px-3 py-2 rounded-lg text-sm">
                        {breakdownWarning}
                    </div>
                )}
                {createError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                        {createError}
                    </div>
                )}

                {!breakdownLoading && (
                    <div className="space-y-3 max-h-80 overflow-y-auto">
                        {groups.map((group) => (
                            <div key={group.id} className="space-y-2 rounded-md border border-input p-3 bg-muted/40">
                                <div className="flex items-start gap-2">
                                    <div className="flex-1 space-y-2">
                                        <input
                                            type="text"
                                            value={group.title}
                                            onChange={(e) => updateGroup(group.id, { title: e.target.value })}
                                            placeholder="Feladat címe"
                                            className="w-full px-2 py-1.5 border border-input rounded-md text-sm font-semibold focus:outline-none focus:ring-2 focus:ring-ring"
                                        />
                                        <textarea
                                            value={group.description}
                                            onChange={(e) => updateGroup(group.id, { description: e.target.value })}
                                            rows={3}
                                            placeholder="Leírás"
                                            className="w-full px-2 py-1.5 border border-input rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring resize-none"
                                        />
                                    </div>
                                    <button
                                        type="button"
                                        onClick={() => deleteGroup(group.id)}
                                        className="p-1.5 text-muted-foreground hover:text-destructive transition-colors"
                                        title="Feladat törlése"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                </div>

                                {group.subtasks.length > 0 && (
                                    <div className="pl-4 space-y-2 border-l-2 border-border">
                                        {group.subtasks.map((sub) => (
                                            <div key={sub.id} className="flex items-start gap-2">
                                                <div className="flex-1 space-y-1.5">
                                                    <input
                                                        type="text"
                                                        value={sub.title}
                                                        onChange={(e) => updateSubtask(group.id, sub.id, { title: e.target.value })}
                                                        placeholder="Al-feladat címe"
                                                        className="w-full px-2 py-1 border border-input rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                                    />
                                                    <textarea
                                                        value={sub.description}
                                                        onChange={(e) => updateSubtask(group.id, sub.id, { description: e.target.value })}
                                                        rows={2}
                                                        placeholder="Leírás"
                                                        className="w-full px-2 py-1 border border-input rounded-md text-xs focus:outline-none focus:ring-2 focus:ring-ring resize-none"
                                                    />
                                                </div>
                                                <button
                                                    type="button"
                                                    onClick={() => deleteSubtask(group.id, sub.id)}
                                                    className="p-1 text-muted-foreground hover:text-destructive transition-colors"
                                                    title="Al-feladat törlése"
                                                >
                                                    <Trash2 size={14} />
                                                </button>
                                            </div>
                                        ))}
                                    </div>
                                )}

                                <button
                                    type="button"
                                    onClick={() => addSubtask(group.id)}
                                    className="flex items-center gap-1 text-xs text-primary hover:underline"
                                >
                                    <Plus size={12} /> Al-feladat hozzáadása
                                </button>
                            </div>
                        ))}

                        <button
                            type="button"
                            onClick={addGroup}
                            className="flex items-center gap-1.5 text-sm text-primary hover:underline"
                        >
                            <Plus size={14} /> Új feladat hozzáadása
                        </button>
                    </div>
                )}

                <Select
                    label="Projekt"
                    value={selectedProjectId}
                    onChange={setSelectedProjectId}
                    disabled={projectsLoading}
                    placeholder="Válassz projektet..."
                    options={projects.map((p) => ({ value: String(p.id), label: p.name }))}
                />
                <Select
                    label="Tábla"
                    value={selectedBoardId}
                    onChange={setSelectedBoardId}
                    disabled={!selectedProjectId || boardsLoading}
                    placeholder="Válassz táblát..."
                    options={boards.map((b) => ({ value: b.id, label: b.name }))}
                />
                <Select
                    label="Oszlop"
                    value={selectedColumnId}
                    onChange={setSelectedColumnId}
                    disabled={!selectedBoardId || columnsLoading}
                    placeholder="Válassz oszlopot..."
                    options={columns.map((c) => ({ value: c.id, label: c.title }))}
                />

                <div className="flex gap-3 pt-2">
                    <Button type="button" variant="outline" onClick={onClose} className="flex-1">
                        Mégse
                    </Button>
                    <Button type="button" onClick={handleConfirm} disabled={!canConfirm} className="flex-1">
                        {creating ? 'Létrehozás...' : 'Létrehozás'}
                    </Button>
                </div>
            </div>
        </Modal>
    )
}
