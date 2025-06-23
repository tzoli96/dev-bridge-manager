'use client'

import { User } from '@/types/user'
import { hasPermission } from '@/utils/permissions'

interface AdminTabProps {
    user: User | null
}

export default function AdminTab({ user }: AdminTabProps) {
    const adminCards = [
        {
            title: "System Settings",
            description: "Configure system-wide settings",
            permission: "system.settings",
            action: () => console.log("Open Settings")
        },
        {
            title: "Role Management",
            description: "Manage user roles and permissions",
            permission: "roles.list",
            action: () => console.log("Manage Roles")
        },
        {
            title: "System Logs",
            description: "View system activity and logs",
            permission: "system.logs",
            action: () => console.log("View Logs")
        },
        {
            title: "Statistics",
            description: "System usage and performance metrics",
            permission: null, // Available for all admins
            action: () => console.log("View Stats")
        }
    ]

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-lg font-semibold">Administration</h2>
                <p className="text-gray-600">System settings and administrative tools</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {adminCards
                    .filter(card => !card.permission || hasPermission(user, card.permission))
                    .map(card => (
                        <div key={card.title} className="bg-white rounded-lg shadow p-6">
                            <h3 className="font-medium mb-2">{card.title}</h3>
                            <p className="text-gray-600 text-sm mb-4">{card.description}</p>
                            <button
                                onClick={card.action}
                                className="bg-purple-600 text-white px-4 py-2 rounded-md hover:bg-purple-700 transition-colors"
                            >
                                Open {card.title.split(' ')[0]}
                            </button>
                        </div>
                    ))}
            </div>
        </div>
    )
}