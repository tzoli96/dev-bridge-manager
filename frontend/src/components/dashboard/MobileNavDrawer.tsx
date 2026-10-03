'use client'

import { useEffect, type RefObject } from 'react'
import Link from 'next/link'
import { Dialog as DialogPrimitive } from 'radix-ui'
import { X } from 'lucide-react'
import { DialogOverlay, DialogPortal, DialogTitle } from '@/components/ui/dialog'
import NavBadge from '@/components/dashboard/NavBadge'
import { isNavActive } from '@/components/dashboard/navActive'
import type { NavGroup } from '@/components/dashboard/navigation'

interface MobileNavDrawerProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    groups: NavGroup[]
    pathname: string
    returnFocusRef: RefObject<HTMLButtonElement | null>
}

export default function MobileNavDrawer({ open, onOpenChange, groups, pathname, returnFocusRef }: MobileNavDrawerProps) {
    useEffect(() => {
        if (!open) return
        const mq = window.matchMedia('(min-width: 1280px)')
        const close = () => {
            if (mq.matches) onOpenChange(false)
        }
        close()
        mq.addEventListener('change', close)
        return () => mq.removeEventListener('change', close)
    }, [open, onOpenChange])

    return (
        <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
            <DialogPortal>
                <DialogOverlay />
                <DialogPrimitive.Content
                    aria-describedby={undefined}
                    onCloseAutoFocus={(event) => {
                        event.preventDefault()
                        returnFocusRef.current?.focus()
                    }}
                    className="fixed inset-y-0 left-0 z-50 flex w-72 max-w-[85vw] flex-col gap-4 overflow-y-auto border-r border-border bg-card p-4 shadow-xl outline-none duration-200 xl:hidden data-[state=open]:animate-in data-[state=open]:slide-in-from-left data-[state=closed]:animate-out data-[state=closed]:slide-out-to-left"
                >
                    <div className="flex items-center justify-between">
                        <DialogTitle className="text-base font-semibold text-foreground">Menü</DialogTitle>
                        <DialogPrimitive.Close
                            aria-label="Menü bezárása"
                            className="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                        >
                            <X size={18} />
                        </DialogPrimitive.Close>
                    </div>

                    <nav aria-label="Fő navigáció" className="flex flex-col gap-5">
                        {groups.map((group) => (
                            <div key={group.id}>
                                <p className="px-3 pb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                    {group.label}
                                </p>
                                <ul className="flex flex-col gap-0.5">
                                    {group.items.map((item) => {
                                        const isActive = isNavActive(pathname, item)
                                        const Icon = item.icon
                                        return (
                                            <li key={item.href}>
                                                <Link
                                                    href={item.href}
                                                    onClick={() => onOpenChange(false)}
                                                    aria-current={isActive ? 'page' : undefined}
                                                    className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                                                        isActive
                                                            ? 'bg-primary/10 text-primary'
                                                            : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                                                    }`}
                                                >
                                                    <Icon size={18} />
                                                    <span className="flex-1">{item.label}</span>
                                                    {item.badge !== undefined && <NavBadge count={item.badge} />}
                                                </Link>
                                            </li>
                                        )
                                    })}
                                </ul>
                            </div>
                        ))}
                    </nav>
                </DialogPrimitive.Content>
            </DialogPortal>
        </DialogPrimitive.Root>
    )
}
