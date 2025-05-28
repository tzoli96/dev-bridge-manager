'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import LoginForm from '../../components/auth/LoginForm'
import RegisterForm from '../../components/auth/RegisterForm'

export default function AuthPage() {
    const [isLogin, setIsLogin] = useState(true)
    const router = useRouter()

    const handleAuthSuccess = () => {
        router.push('/dashboard')
    }

    return (
        <div className="min-h-screen bg-gray-50 flex items-center justify-center py-12 px-4">
            <div className="w-full max-w-md">
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