export default function UnauthorizedPage() {
    const router = useRouter();

    return (
        <div className="min-h-screen bg-gray-50 flex items-center justify-center">
            <div className="max-w-md mx-auto text-center">
                <div className="bg-white rounded-lg shadow-md p-8">
                    <div className="w-16 h-16 mx-auto mb-4 text-red-500">
                        <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.5 0L4.268 18.5c-.77.833.192 2.5 1.732 2.5z" />
                        </svg>
                    </div>
                    <h1 className="text-2xl font-bold text-gray-900 mb-4">
                        Access Denied
                    </h1>
                    <p className="text-gray-600 mb-6">
                        You don't have permission to access this page.
                    </p>
                    <div className="space-y-3">
                        <button
                            onClick={() => router.back()}
                            className="w-full bg-gray-600 text-white px-4 py-2 rounded-md hover:bg-gray-700"
                        >
                            Go Back
                        </button>
                        <button
                            onClick={() => router.push('/dashboard')}
                            className="w-full bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700"
                        >
                            Go to Dashboard
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}