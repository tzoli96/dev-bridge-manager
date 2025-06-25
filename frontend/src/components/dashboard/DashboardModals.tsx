'use client'

import { useState } from 'react'
import EditProfileModal from '@/components/EditProfileModal'
import ChangePasswordModal from '@/components/ChangePasswordModal'
import CreateUserModal from '@/components/CreateUserModal'
import EditUserModal from '@/components/EditUserModal'
import CreateProjectModal from '@/components/CreateProjectModal'
import EditProjectModal from '@/components/EditProjectModal'
import { User } from '@/types/user'
import { Project } from '@/services/projectsService'

// Modal state management with Context API
import { createContext, useContext, ReactNode } from 'react'

interface ModalContextType {
    // Profile modals
    showEditProfile: boolean
    setShowEditProfile: (show: boolean) => void
    showChangePassword: boolean
    setShowChangePassword: (show: boolean) => void

    // User modals
    showCreateUser: boolean
    setShowCreateUser: (show: boolean) => void
    showEditUser: boolean
    setShowEditUser: (show: boolean) => void
    selectedUser: User | null
    setSelectedUser: (user: User | null) => void

    // Project modals
    showCreateProject: boolean
    setShowCreateProject: (show: boolean) => void
    showEditProject: boolean
    setShowEditProject: (show: boolean) => void
    selectedProject: Project | null
    setSelectedProject: (project: Project | null) => void

    // Callback functions for refreshing data
    onProjectUpdated?: () => void
    setOnProjectUpdated: (callback: (() => void) | undefined) => void
    onUserUpdated?: () => void
    setOnUserUpdated: (callback: (() => void) | undefined) => void
}

const ModalContext = createContext<ModalContextType | null>(null)

export const useModals = () => {
    const context = useContext(ModalContext)
    if (!context) throw new Error('useModals must be used within ModalProvider')
    return context
}

export function ModalProvider({ children }: { children: ReactNode }) {
    const [showEditProfile, setShowEditProfile] = useState(false)
    const [showChangePassword, setShowChangePassword] = useState(false)
    const [showCreateUser, setShowCreateUser] = useState(false)
    const [showEditUser, setShowEditUser] = useState(false)
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [showCreateProject, setShowCreateProject] = useState(false)
    const [showEditProject, setShowEditProject] = useState(false)
    const [selectedProject, setSelectedProject] = useState<Project | null>(null)

    // Callback functions for data refreshing
    const [onProjectUpdated, setOnProjectUpdated] = useState<(() => void) | undefined>(undefined)
    const [onUserUpdated, setOnUserUpdated] = useState<(() => void) | undefined>(undefined)

    return (
        <ModalContext.Provider value={{
            showEditProfile, setShowEditProfile,
            showChangePassword, setShowChangePassword,
            showCreateUser, setShowCreateUser,
            showEditUser, setShowEditUser,
            selectedUser, setSelectedUser,
            showCreateProject, setShowCreateProject,
            showEditProject, setShowEditProject,
            selectedProject, setSelectedProject,
            onProjectUpdated, setOnProjectUpdated,
            onUserUpdated, setOnUserUpdated
        }}>
            {children}
        </ModalContext.Provider>
    )
}

export default function DashboardModals() {
    const {
        showEditProfile, setShowEditProfile,
        showChangePassword, setShowChangePassword,
        showCreateUser, setShowCreateUser,
        showEditUser, setShowEditUser, selectedUser, setSelectedUser,
        showCreateProject, setShowCreateProject,
        showEditProject, setShowEditProject, selectedProject, setSelectedProject,
        onProjectUpdated, onUserUpdated
    } = useModals()

    return (
        <>
            <EditProfileModal
                isOpen={showEditProfile}
                onClose={() => setShowEditProfile(false)}
                onSuccess={() => console.log('Profile updated')}
            />

            <ChangePasswordModal
                isOpen={showChangePassword}
                onClose={() => setShowChangePassword(false)}
                onSuccess={() => console.log('Password changed')}
            />

            <CreateUserModal
                isOpen={showCreateUser}
                onClose={() => setShowCreateUser(false)}
                onSuccess={() => {
                    console.log('User created')
                    if (onUserUpdated) onUserUpdated()
                }}
            />

            <EditUserModal
                isOpen={showEditUser}
                user={selectedUser}
                onClose={() => {
                    setShowEditUser(false)
                    setSelectedUser(null)
                }}
                onSuccess={() => {
                    console.log('User updated')
                    if (onUserUpdated) onUserUpdated()
                }}
            />

            <CreateProjectModal
                isOpen={showCreateProject}
                onClose={() => setShowCreateProject(false)}
                onSuccess={() => {
                    console.log('Project created')
                    if (onProjectUpdated) onProjectUpdated()
                }}
            />

            <EditProjectModal
                isOpen={showEditProject}
                project={selectedProject}
                onClose={() => {
                    setShowEditProject(false)
                    setSelectedProject(null)
                }}
                onSuccess={() => {
                    console.log('Project updated')
                    if (onProjectUpdated) onProjectUpdated()
                }}
            />
        </>
    )
}