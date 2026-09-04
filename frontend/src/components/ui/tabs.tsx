"use client"

import * as React from "react"
import { cn } from "cn"
import { Tabs as TabsPrimitive } from "radix-ui"
import type { LucideIcon } from "lucide-react"

export interface TabItem {
  id: string
  label: string
  icon?: LucideIcon
}

interface TabsProps {
  tabs: TabItem[]
  activeTab: string
  onChange: (id: string) => void
  className?: string
}

export function Tabs({ tabs, activeTab, onChange, className }: TabsProps) {
  return (
    <TabsPrimitive.Root
      value={activeTab}
      onValueChange={onChange}
      className={cn("w-full", className)}
    >
      <TabsPrimitive.List className="flex gap-1 border-b px-6">
        {tabs.map(({ id, label, icon: Icon }) => (
          <TabsPrimitive.Trigger
            key={id}
            value={id}
            className={cn(
              "flex items-center gap-1.5 border-b-2 border-transparent px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground",
              "data-active:border-primary data-active:text-foreground"
            )}
          >
            {Icon && <Icon className="size-4" />}
            {label}
          </TabsPrimitive.Trigger>
        ))}
      </TabsPrimitive.List>
    </TabsPrimitive.Root>
  )
}
