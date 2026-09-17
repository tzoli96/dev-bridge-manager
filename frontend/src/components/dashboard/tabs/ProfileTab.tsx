'use client'

import { User } from '@/types/user'
import { useModals } from '@/components/dashboard/DashboardModals'
import { hasPermission } from '@/utils/permissions'

interface ProfileTabProps {
    user: User | null
}

export default function ProfileTab({ user }: ProfileTabProps) {
    const { setShowEditProfile, setShowChangePassword } = useModals()

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-lg font-semibold text-foreground">Your Profile</h2>
                <p className="text-muted-foreground text-sm">View and manage your account information</p>
            </div>

            <div className="bg-card rounded-xl shadow-sm border border-border p-6">
                <h3 className="text-base font-semibold text-foreground mb-4">Account Information</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <label className="block text-sm font-medium text-foreground">Name</label>
                        <p className="mt-1 text-foreground">{user?.name}</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-foreground">Email</label>
                        <p className="mt-1 text-foreground">{user?.email}</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-foreground">Position</label>
                        <p className="mt-1 text-foreground">{user?.position || 'Not specified'}</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-foreground">Role</label>
                        <p className="mt-1 text-foreground">{user?.role?.display_name}</p>
                    </div>
                </div>

                {hasPermission(user, 'profile.update') && (
                    <div className="mt-6 space-x-3">
                        <button
                            onClick={() => setShowEditProfile(true)}
                            className="bg-primary text-white px-4 py-2 rounded-lg font-medium text-sm shadow-sm hover:bg-primary/90 hover:shadow-sm active:scale-[0.98] transition-all"
                        >
                            Edit Profile
                        </button>
                        <button
                            onClick={() => setShowChangePassword(true)}
                            className="bg-card text-destructive border border-destructive/20 px-4 py-2 rounded-lg font-medium text-sm hover:bg-destructive/10 active:scale-[0.98] transition-all"
                        >
                            Change Password
                        </button>
                    </div>
                )}
            </div>

            {/* Debug info csak fejlesztési módban */}
            {process.env.NODE_ENV === 'development' && user?.permissions && (
                <div className="bg-card rounded-xl shadow-sm border border-border p-6">
                    <h3 className="text-base font-semibold text-foreground mb-4">Your Permissions (Debug)</h3>
                    <div className="flex flex-wrap gap-2">
                        {user.permissions.map((permission: string) => (
                            <span
                                key={permission}
                                className="text-xs bg-success/10 text-success px-2 py-1 rounded"
                            >
                                {permission}
                            </span>
                        ))}
                    </div>
                </div>
            )}
        </div>
    )
}