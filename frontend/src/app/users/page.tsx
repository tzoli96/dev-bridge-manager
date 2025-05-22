import UsersList from '@/components/UsersList'

export default function UsersPage() {
    return (
        <div className="min-h-screen bg-gray-50">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <UsersList />
            </div>
        </div>
    )
}

export const metadata = {
    title: 'Users - Dev Bridge Manager',
    description: 'Team members and users list'
}