import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import UsersPageContent from './UsersPageContent'

export default function UsersPage() {
    return (
        <ProtectedRoute
            permissions={['users.list', 'users.read']}
            requireAll={false}
        >
            <div className="min-h-screen bg-gray-50">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                    <UsersPageContent />
                </div>
            </div>
        </ProtectedRoute>
    )
}