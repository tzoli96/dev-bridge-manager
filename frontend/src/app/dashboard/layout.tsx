import { ModalProvider } from '@/components/dashboard/DashboardModals'

export default function DashboardLayout({
                                            children,
                                        }: {
    children: React.ReactNode
}) {
    return (
        <ModalProvider>
            {children}
        </ModalProvider>
    )
}