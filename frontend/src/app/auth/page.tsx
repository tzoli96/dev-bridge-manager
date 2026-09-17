'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Boxes } from 'lucide-react'
import LoginForm from '../../components/auth/LoginForm'
import RegisterForm from '../../components/auth/RegisterForm'

export default function AuthPage() {
    const [isLogin, setIsLogin] = useState(true)
    const router = useRouter()

    const handleAuthSuccess = () => {
        router.push('/dashboard')
    }

    return (
        <div className="min-h-screen flex items-center justify-center bg-background py-12 px-4">
            <div className="w-full max-w-md">
                <div className="mb-8 flex flex-col items-center text-center">
                    <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-white shadow-sm shadow-primary/20">
                        <Boxes size={24} />
                    </div>
                    <h1 className="text-xl font-semibold text-foreground">Dev Bridge Manager</h1>
                    <p className="mt-1 text-sm text-muted-foreground">Multi-agency development project management</p>
                </div>

                {isLogin ? (
                    <LoginForm
                        onSuccess={handleAuthSuccess}
                        onToggleForm={() => setIsLogin(false)}
                    />
                ) : (
                    <RegisterForm
                        onSuccess={handleAuthSuccess}
                        onToggleForm={() => setIsLogin(true)}
                    />
                )}
            </div>
        </div>
    )
}
