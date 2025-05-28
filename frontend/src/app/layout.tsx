import { Metadata } from 'next'
import { AuthProvider } from '../contexts/AuthContext'
import './globals.css'

export const metadata: Metadata = {
    title: 'Dev Bridge Manager',
    description: 'Multi-agency development project management',
}

export default function RootLayout({
                                       children,
                                   }: {
    children: React.ReactNode
}) {
    return (
        <html lang="en">
        <body>
        <AuthProvider>
            {children}
        </AuthProvider>
        </body>
        </html>
    )
}