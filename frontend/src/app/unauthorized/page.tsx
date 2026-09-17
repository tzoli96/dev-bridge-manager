'use client'

import { useRouter } from 'next/navigation'
import { ShieldAlert } from 'lucide-react'

export default function UnauthorizedPage() {
    const router = useRouter();

    return (
        <div className="min-h-screen bg-muted flex items-center justify-center px-4">
            <div className="max-w-md mx-auto text-center">
                <div className="bg-card rounded-xl shadow-sm border border-border p-8">
                    <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-destructive/10 text-destructive">
                        <ShieldAlert size={28} />
                    </div>
                    <h1 className="text-2xl font-bold text-foreground mb-2">
                        Access Denied
                    </h1>
                    <p className="text-muted-foreground mb-6">
                        You don&apos;t have permission to access this page.
                    </p>
                    <div className="space-y-3">
                        <button
                            onClick={() => router.back()}
                            className="w-full bg-card text-foreground border border-border px-4 py-2.5 rounded-lg font-medium text-sm hover:bg-muted transition-colors"
                        >
                            Go Back
                        </button>
                        <button
                            onClick={() => router.push('/dashboard')}
                            className="w-full bg-primary text-white px-4 py-2.5 rounded-lg font-medium text-sm shadow-sm hover:bg-primary/90 hover:shadow-sm transition-all"
                        >
                            Go to Dashboard
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}