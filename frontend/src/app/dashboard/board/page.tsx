'use client'

import { useAuth } from '@/contexts/AuthContext'
import BoardTab from '@/components/dashboard/tabs/BoardTab'

export default function BoardPage() {
    const { user } = useAuth()

    return <BoardTab user={user} />
}