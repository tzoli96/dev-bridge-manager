import { Invoice } from '@/services/invoicesService'

export function computeInvoiceStats(invoices: Invoice[]) {
    const createdInvoices = invoices.filter((inv) => inv.status === 'created')
    const createdInvoicesTotal = createdInvoices.reduce((sum, inv) => sum + inv.amount, 0)
    const failedInvoicesCount = invoices.length - createdInvoices.length
    const avgInvoiceAmount = createdInvoices.length > 0 ? createdInvoicesTotal / createdInvoices.length : 0

    const paidTotal = createdInvoices.filter((inv) => inv.payment_status === 'paid').reduce((s, inv) => s + inv.amount, 0)
    const partiallyPaidTotal = createdInvoices.filter((inv) => inv.payment_status === 'partially_paid').reduce((s, inv) => s + inv.amount, 0)
    const expiredTotal = createdInvoices.filter((inv) => inv.payment_status === 'expired').reduce((s, inv) => s + inv.amount, 0)
    const outstandingTotal = Math.max(createdInvoicesTotal - paidTotal - partiallyPaidTotal - expiredTotal, 0)

    const todayKey = new Date().toISOString().slice(0, 10)
    const overdueCount = createdInvoices.filter(
        (inv) => inv.due_date && inv.due_date.slice(0, 10) < todayKey && inv.payment_status !== 'paid'
    ).length

    const clientMap = new Map<string, number>()
    for (const inv of createdInvoices) {
        clientMap.set(inv.client_name, (clientMap.get(inv.client_name) || 0) + inv.amount)
    }
    const revenueByClient = Array.from(clientMap.entries())
        .map(([name, amount]) => ({ name, amount }))
        .sort((a, b) => b.amount - a.amount)
    const maxClientRevenue = Math.max(...revenueByClient.map((c) => c.amount), 1)

    return {
        createdInvoices,
        createdInvoicesTotal,
        failedInvoicesCount,
        avgInvoiceAmount,
        paidTotal,
        partiallyPaidTotal,
        expiredTotal,
        outstandingTotal,
        overdueCount,
        revenueByClient,
        maxClientRevenue,
    }
}
