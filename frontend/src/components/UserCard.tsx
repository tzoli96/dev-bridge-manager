import { User } from '@/types/user'

interface UserCardProps {
    user: User
}

export default function UserCard({ user }: UserCardProps) {
    const formatDate = (dateString: string) => {
        return new Date(dateString).toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric'
        })
    }

    return (
        <div className="bg-card rounded-lg shadow-sm p-6 hover:shadow-sm transition-shadow">
            <div className="flex items-start justify-between">
                <div className="flex-1">
                    <h3 className="text-lg font-semibold text-foreground mb-1">
                        {user.name}
                    </h3>
                    <p className="text-sm text-muted-foreground mb-2">
                        {user.position}
                    </p>
                    <p className="text-sm text-primary hover:text-primary/80">
                        {user.email}
                    </p>
                </div>
                <div className="flex flex-col items-end text-xs text-muted-foreground">
                    <span>ID: {user.id}</span>
                    <span>Created: {formatDate(user.created_at)}</span>
                </div>
            </div>
        </div>
    )
}