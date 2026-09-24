'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { User } from '@/types/user'
import { hasPermission, isSuperAdmin } from '@/utils/permissions'
import BillingoSettingsModal from '@/components/BillingoSettingsModal'
import ProfileModal from '@/components/ProfileModal'

interface AdminTabProps {
    user: User | null
}

export default function AdminTab({ user }: AdminTabProps) {
    const router = useRouter()
    const [isBillingoModalOpen, setIsBillingoModalOpen] = useState(false)
    const [isProfileModalOpen, setIsProfileModalOpen] = useState(false)

    const adminCards = [
        {
            title: "Billingo Integration",
            description: "Configure the Billingo API key and invoicing block used for automatic invoicing",
            buttonLabel: "Configure",
            permission: "billingo_settings.manage",
            action: () => setIsBillingoModalOpen(true),
            comingSoon: false
        },
        {
            title: "Role Management",
            description: "Manage user roles and permissions",
            buttonLabel: "Manage Roles",
            permission: "roles.list",
            action: () => {},
            comingSoon: true
        },
        {
            title: "System Logs",
            description: "View system activity and logs",
            buttonLabel: "View Logs",
            permission: "system.logs",
            action: () => {},
            comingSoon: true
        },
        {
            title: "Statistics",
            description: "System usage and performance metrics",
            buttonLabel: "View Stats",
            permission: null, // Available for all admins
            action: () => {},
            comingSoon: true
        },
        {
            title: "AI Profil / Perszóna",
            description: "Háttér, szakterület és írásminták beállítása, amit az AI-alapú funkciók a te stílusodban való válaszadáshoz használnak",
            buttonLabel: "Szerkesztés",
            permission: null,
            requireSuperAdmin: true,
            action: () => setIsProfileModalOpen(true),
            comingSoon: false
        },
        {
            title: "Álláskeresés",
            description: "profession.hu automatikus figyelése, AI-alapú illeszkedés-értékelés és jelentkezési tervezet",
            buttonLabel: "Megnyitás",
            permission: null,
            requireSuperAdmin: true,
            action: () => router.push('/dashboard/admin/job-search'),
            comingSoon: false
        }
    ]

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-lg font-semibold text-foreground">Administration</h2>
                <p className="text-muted-foreground text-sm">System settings and administrative tools</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {adminCards
                    .filter(card => (card as any).requireSuperAdmin ? isSuperAdmin(user) : (!card.permission || hasPermission(user, card.permission)))
                    .map(card => (
                        <div key={card.title} className="bg-card rounded-xl shadow-sm border border-border p-6 transition-shadow hover:shadow-sm">
                            <div className="flex items-center gap-2 mb-1">
                                <h3 className="font-medium text-foreground">{card.title}</h3>
                                {card.comingSoon && (
                                    <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                                        Coming soon
                                    </span>
                                )}
                            </div>
                            <p className="text-muted-foreground text-sm mb-4">{card.description}</p>
                            <button
                                onClick={card.action}
                                disabled={card.comingSoon}
                                className="bg-primary text-white px-4 py-2 rounded-lg font-medium text-sm shadow-sm hover:bg-primary/90 hover:shadow-sm active:scale-[0.98] transition-all disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100"
                            >
                                {card.buttonLabel}
                            </button>
                        </div>
                    ))}
            </div>

            <BillingoSettingsModal
                isOpen={isBillingoModalOpen}
                onClose={() => setIsBillingoModalOpen(false)}
            />
            <ProfileModal
                isOpen={isProfileModalOpen}
                onClose={() => setIsProfileModalOpen(false)}
            />
        </div>
    )
}
