interface ErrorStateProps {
    error: string
    onRetry?: () => void
}

export default function ErrorState({ error, onRetry }: ErrorStateProps) {
    return (
        <div className="space-y-4">
            <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded">
                <div className="font-medium">Error occurred:</div>
                <div>{error}</div>
            </div>
            {onRetry && (
                <button
                    onClick={onRetry}
                    className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                >
                    Try Again
                </button>
            )}
        </div>
    )
}