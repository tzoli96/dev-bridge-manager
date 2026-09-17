'use client'

import { LogOut, Boxes } from 'lucide-react'
import { User } from '@/types/user'
import { ThemeToggle } from '@/components/theme-toggle'

interface DashboardHeaderProps {
    user: User | null
    onLogout: () => void
}

export default function DashboardHeader({ user, onLogout }: DashboardHeaderProps) {
    const initial = user?.name?.charAt(0).toUpperCase() || '?'

    return (
        <div className="bg-card border-b border-border">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="flex justify-between items-center py-4">
                    <div className="flex items-center gap-3">
                        <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-white shadow-sm shadow-primary/20">
                            <Boxes size={18} />
                        </div>
                        <div>
                            <h1 className="text-base font-semibold text-foreground leading-tight">Dev Bridge Manager</h1>
                            <p className="text-xs text-muted-foreground">Project management</p>
                        </div>
                    </div>

                    <div className="flex items-center gap-3">
                        <div className="hidden sm:flex items-center gap-2.5 pr-3 border-r border-border">
                            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary text-sm font-semibold">
                                {initial}
                            </div>
                            <div className="text-sm">
                                <div className="font-medium text-foreground leading-tight">{user?.name}</div>
                                <div className="text-xs text-muted-foreground">{user?.role?.display_name}</div>
                            </div>
                        </div>
                        <ThemeToggle />
                        <button
                            onClick={onLogout}
                            className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-destructive px-3 py-2 rounded-lg hover:bg-destructive/10 transition-colors"
                        >
                            <LogOut size={16} />
                            <span className="hidden sm:inline">Logout</span>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
