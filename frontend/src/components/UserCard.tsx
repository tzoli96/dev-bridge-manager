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
        <div className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
            <div className="flex items-start justify-between">
                <div className="flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 mb-1">
                        {user.name}
                    </h3>
                    <p className="text-sm text-gray-600 mb-2">
                        {user.position}
                    </p>
                    <p className="text-sm text-blue-600 hover:text-blue-800">
                        {user.email}
                    </p>
                </div>
                <div className="flex flex-col items-end text-xs text-gray-400">
                    <span>ID: {user.id}</span>
                    <span>Created: {formatDate(user.created_at)}</span>
                </div>
            </div>
        </div>
    )
}