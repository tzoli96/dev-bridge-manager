import { useState, useEffect } from 'react'
import { Project } from '@/services/projectsService'
import { ProjectAssignmentService, ProjectAssignment } from '@/services/projectAssignmentService'
import { UsersService } from '@/services/usersService'
import { User, User as CurrentUser } from '@/types/user'
import { isAdmin } from '@/utils/permissions'

interface ProjectAssignmentModalProps {
    isOpen: boolean
    project: Project | null
    currentUser: CurrentUser | null
    onClose: () => void
}

// Helper function to check if user is manager or admin
const canManageTeam = (user: CurrentUser | null): boolean => {
    if (!user) return false
    // Ellenőrizzük role_id alapján is, mivel a role object üres lehet
    return isAdmin(user) || user.role?.name === 'manager' || user.role_id === 3 // 3 = manager role_id
}

export default function ProjectAssignmentModal({ isOpen, project, currentUser, onClose }: ProjectAssignmentModalProps) {
    const [assignments, setAssignments] = useState<ProjectAssignment[]>([])
    const [allUsers, setAllUsers] = useState<User[]>([])
    const [availableUsers, setAvailableUsers] = useState<User[]>([])
    const [loading, setLoading] = useState(true)
    const [actionLoading, setActionLoading] = useState<number | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [showAddUser, setShowAddUser] = useState(false)
    const [selectedUserId, setSelectedUserId] = useState<number | null>(null)
    const [selectedRole, setSelectedRole] = useState<'owner' | 'manager' | 'member' | 'viewer'>('member')

    const canManageAssignments = canManageTeam(currentUser)

    const fetchData = async () => {
        if (!project) return

        try {
            setLoading(true)
            setError(null)

            const [assignmentsData, usersData] = await Promise.all([
                ProjectAssignmentService.getProjectAssignments(project.id),
                UsersService.getAllUsers()
            ])


            setAssignments(assignmentsData || [])
            setAllUsers(usersData || [])

            // Available users = users not assigned to this project
            const assignedUserIds = new Set((assignmentsData || []).map(a => a.user_id))
            const available = (usersData || []).filter(user => !assignedUserIds.has(user.id))

            console.log('🔍 [ProjectAssignmentModal] Available users:', available)
            console.log('🔍 [ProjectAssignmentModal] Can manage assignments:', canManageAssignments)
            console.log('🔍 [ProjectAssignmentModal] Current user:', currentUser)

            setAvailableUsers(available)
        } catch (err: any) {
            console.error('❌ [ProjectAssignmentModal] Error fetching data:', err)
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        if (isOpen && project) {
            fetchData()
        }
    }, [isOpen, project])

    const handleAssignUser = async () => {
        if (!project || !selectedUserId) return

        try {
            setActionLoading(selectedUserId)
            await ProjectAssignmentService.assignUserToProject(project.id, {
                user_id: selectedUserId,
                role: selectedRole
            })

            setShowAddUser(false)
            setSelectedUserId(null)
            setSelectedRole('member')
            await fetchData()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setActionLoading(null)
        }
    }

    const handleUpdateRole = async (userId: number, newRole: 'owner' | 'manager' | 'member' | 'viewer') => {
        if (!project) return

        try {
            setActionLoading(userId)
            await ProjectAssignmentService.updateProjectAssignment(project.id, userId, { role: newRole })
            await fetchData()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setActionLoading(null)
        }
    }

    const handleRemoveUser = async (userId: number, userName: string) => {
        if (!project || !confirm(`Are you sure you want to remove ${userName} from this project?`)) return

        try {
            setActionLoading(userId)
            await ProjectAssignmentService.removeUserFromProject(project.id, userId)
            await fetchData()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setActionLoading(null)
        }
    }

    const getRoleColor = (role: string) => {
        switch (role) {
            case 'owner': return 'bg-primary/10 text-primary'
            case 'manager': return 'bg-primary/10 text-primary'
            case 'member': return 'bg-success/10 text-success'
            case 'viewer': return 'bg-muted text-foreground'
            default: return 'bg-muted text-foreground'
        }
    }

    if (!isOpen || !project) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
            <div className="bg-card rounded-xl shadow-sm w-full max-w-2xl max-h-[90vh] flex flex-col">
                {/* Header */}
                <div className="flex justify-between items-center p-6 border-b border-border">
                    <div>
                        <h2 className="text-xl font-semibold text-foreground">Team Members</h2>
                        <p className="text-muted-foreground text-sm">{project.name}</p>
                    </div>
                    <button
                        onClick={onClose}
                        className="flex items-center justify-center h-8 w-8 rounded-lg text-muted-foreground hover:text-muted-foreground hover:bg-muted transition-colors"
                    >
                        ✕
                    </button>
                </div>

                {/* Content */}
                <div className="flex-1 overflow-y-auto p-6">
                    {error && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-4">
                            {error}
                        </div>
                    )}

                    {loading ? (
                        <div className="flex items-center justify-center py-8">
                            <div className="text-muted-foreground">Loading team members...</div>
                        </div>
                    ) : (
                        <div className="space-y-6">
                            {/* Current Team Members */}
                            <div>
                                <div className="flex justify-between items-center mb-4">
                                    <h3 className="font-medium text-foreground">
                                        Current Members ({assignments.length})
                                    </h3>

                                    {/* Debug info */}
                                    <div className="text-xs text-muted-foreground">
                                        Available: {availableUsers.length} | Can manage: {canManageAssignments ? 'Yes' : 'No'}
                                    </div>

                                    {canManageAssignments && availableUsers.length > 0 && (
                                        <button
                                            onClick={() => setShowAddUser(true)}
                                            className="bg-primary text-white px-3 py-2 rounded-lg hover:bg-primary/90 text-sm font-medium"
                                        >
                                            + Add Member
                                        </button>
                                    )}

                                    {/* Fallback button ha nincs available user de van manage jog */}
                                    {canManageAssignments && availableUsers.length === 0 && allUsers.length > 0 && (
                                        <div className="text-sm text-muted-foreground">
                                            All users assigned
                                        </div>
                                    )}
                                </div>

                                {assignments.length === 0 ? (
                                    <div className="text-center py-8 text-muted-foreground">
                                        No team members assigned yet.
                                    </div>
                                ) : (
                                    <div className="space-y-3">
                                        {assignments.map((assignment) => (
                                            <div key={assignment.id} className="flex items-center justify-between p-3 bg-muted rounded-lg">
                                                <div className="flex items-center space-x-3">
                                                    <div className="w-8 h-8 bg-primary text-white rounded-full flex items-center justify-center text-sm font-medium">
                                                        {assignment.user_name.charAt(0).toUpperCase()}
                                                    </div>
                                                    <div>
                                                        <div className="font-medium text-foreground">{assignment.user_name}</div>
                                                        <div className="text-sm text-muted-foreground">{assignment.user_email}</div>
                                                    </div>
                                                </div>

                                                <div className="flex items-center space-x-3">
                                                    {canManageAssignments ? (
                                                        <select
                                                            value={assignment.role}
                                                            onChange={(e) => handleUpdateRole(assignment.user_id, e.target.value as any)}
                                                            disabled={actionLoading === assignment.user_id}
                                                            className="text-sm border border-input rounded px-2 py-1 focus:outline-none focus:ring-2 focus:ring-ring"
                                                        >
                                                            <option value="owner">Owner</option>
                                                            <option value="manager">Manager</option>
                                                            <option value="member">Member</option>
                                                            <option value="viewer">Viewer</option>
                                                        </select>
                                                    ) : (
                                                        <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${getRoleColor(assignment.role)}`}>
                                                            {assignment.role}
                                                        </span>
                                                    )}

                                                    {canManageAssignments && (
                                                        <button
                                                            onClick={() => handleRemoveUser(assignment.user_id, assignment.user_name)}
                                                            disabled={actionLoading === assignment.user_id}
                                                            className="text-destructive hover:text-destructive text-sm disabled:opacity-50"
                                                        >
                                                            {actionLoading === assignment.user_id ? 'Removing...' : 'Remove'}
                                                        </button>
                                                    )}
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>

                            {/* Add New Member Form */}
                            {showAddUser && canManageAssignments && (
                                <div className="border-t pt-6">
                                    <h3 className="font-medium text-foreground mb-4">Add New Member</h3>
                                    <div className="space-y-4">
                                        <div>
                                            <label className="block text-sm font-medium text-foreground mb-2">
                                                Select User
                                            </label>
                                            <select
                                                value={selectedUserId || ''}
                                                onChange={(e) => setSelectedUserId(Number(e.target.value))}
                                                className="w-full border border-input rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-ring"
                                            >
                                                <option value="">Choose a user...</option>
                                                {availableUsers.map((user) => (
                                                    <option key={user.id} value={user.id}>
                                                        {user.name} ({user.email})
                                                    </option>
                                                ))}
                                            </select>
                                        </div>

                                        <div>
                                            <label className="block text-sm font-medium text-foreground mb-2">
                                                Role
                                            </label>
                                            <select
                                                value={selectedRole}
                                                onChange={(e) => setSelectedRole(e.target.value as any)}
                                                className="w-full border border-input rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-ring"
                                            >
                                                <option value="member">Member</option>
                                                <option value="viewer">Viewer</option>
                                                <option value="manager">Manager</option>
                                                <option value="owner">Owner</option>
                                            </select>
                                        </div>

                                        <div className="flex space-x-3">
                                            <button
                                                onClick={() => {
                                                    setShowAddUser(false)
                                                    setSelectedUserId(null)
                                                    setSelectedRole('member')
                                                }}
                                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted transition-colors"
                                            >
                                                Cancel
                                            </button>
                                            <button
                                                onClick={handleAssignUser}
                                                disabled={!selectedUserId || actionLoading === selectedUserId}
                                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                                            >
                                                {actionLoading === selectedUserId ? 'Adding...' : 'Add Member'}
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            )}

                            {/* Available Users Info */}
                            {!showAddUser && canManageAssignments && availableUsers.length === 0 && assignments.length > 0 && (
                                <div className="text-center py-4 text-muted-foreground text-sm">
                                    All users are already assigned to this project.
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}