import { useState } from 'react'
import { ClientsService, ClientCreateRequest } from '@/services/clientsService'

interface CreateClientModalProps {
    isOpen: boolean
    onClose: () => void
    onSuccess: () => void
}

const initialFormData: ClientCreateRequest = {
    type: 'company',
    name: '',
    tax_number: '',
    eu_vat_number: '',
    company_reg_number: '',
    billing_zip: '',
    billing_city: '',
    billing_address: '',
    bank_account_number: '',
    email: '',
    phone: '',
    notes: '',
    billingo_unit: '',
    billingo_unit_price_type: ''
}

export default function CreateClientModal({ isOpen, onClose, onSuccess }: CreateClientModalProps) {
    const [formData, setFormData] = useState<ClientCreateRequest>(initialFormData)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()

        if (!formData.name?.trim()) {
            setError('Client name is required')
            return
        }

        if (formData.type !== 'individual' && !formData.tax_number?.trim()) {
            setError('Tax number is required for company clients')
            return
        }

        try {
            setLoading(true)
            setError(null)

            await ClientsService.createClient({
                ...formData,
                name: formData.name.trim()
            })

            setFormData(initialFormData)
            onSuccess()
            onClose()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    const handleClose = () => {
        if (!loading) {
            setFormData(initialFormData)
            setError(null)
            onClose()
        }
    }

    if (!isOpen) return null

    const isIndividual = formData.type === 'individual'

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-lg">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">New Client</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    {error && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {error}
                        </div>
                    )}

                    <div>
                        <label className="block text-sm font-medium text-foreground mb-1">Type</label>
                        <div className="flex gap-4">
                            <label className="flex items-center gap-2 text-sm">
                                <input
                                    type="radio"
                                    checked={formData.type === 'company'}
                                    onChange={() => setFormData(prev => ({ ...prev, type: 'company' }))}
                                    disabled={loading}
                                />
                                Company
                            </label>
                            <label className="flex items-center gap-2 text-sm">
                                <input
                                    type="radio"
                                    checked={formData.type === 'individual'}
                                    onChange={() => setFormData(prev => ({ ...prev, type: 'individual' }))}
                                    disabled={loading}
                                />
                                Individual
                            </label>
                        </div>
                    </div>

                    <div>
                        <label htmlFor="name" className="block text-sm font-medium text-foreground mb-1">
                            Name / Company Name *
                        </label>
                        <input
                            type="text"
                            id="name"
                            value={formData.name}
                            onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={loading}
                            required
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label htmlFor="tax_number" className="block text-sm font-medium text-foreground mb-1">
                                Tax Number {!isIndividual && '*'}
                            </label>
                            <input
                                type="text"
                                id="tax_number"
                                value={formData.tax_number}
                                onChange={(e) => setFormData(prev => ({ ...prev, tax_number: e.target.value }))}
                                placeholder="12345678-1-42"
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                        <div>
                            <label htmlFor="eu_vat_number" className="block text-sm font-medium text-foreground mb-1">
                                EU VAT Number
                            </label>
                            <input
                                type="text"
                                id="eu_vat_number"
                                value={formData.eu_vat_number}
                                onChange={(e) => setFormData(prev => ({ ...prev, eu_vat_number: e.target.value }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                    </div>

                    {!isIndividual && (
                        <div>
                            <label htmlFor="company_reg_number" className="block text-sm font-medium text-foreground mb-1">
                                Company Registration Number
                            </label>
                            <input
                                type="text"
                                id="company_reg_number"
                                value={formData.company_reg_number}
                                onChange={(e) => setFormData(prev => ({ ...prev, company_reg_number: e.target.value }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                    )}

                    <div className="grid grid-cols-3 gap-4">
                        <div>
                            <label htmlFor="billing_zip" className="block text-sm font-medium text-foreground mb-1">
                                ZIP Code
                            </label>
                            <input
                                type="text"
                                id="billing_zip"
                                value={formData.billing_zip}
                                onChange={(e) => setFormData(prev => ({ ...prev, billing_zip: e.target.value }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                        <div className="col-span-2">
                            <label htmlFor="billing_city" className="block text-sm font-medium text-foreground mb-1">
                                City
                            </label>
                            <input
                                type="text"
                                id="billing_city"
                                value={formData.billing_city}
                                onChange={(e) => setFormData(prev => ({ ...prev, billing_city: e.target.value }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                    </div>

                    <div>
                        <label htmlFor="billing_address" className="block text-sm font-medium text-foreground mb-1">
                            Address
                        </label>
                        <input
                            type="text"
                            id="billing_address"
                            value={formData.billing_address}
                            onChange={(e) => setFormData(prev => ({ ...prev, billing_address: e.target.value }))}
                            placeholder="street, house number"
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={loading}
                        />
                    </div>

                    <div>
                        <label htmlFor="bank_account_number" className="block text-sm font-medium text-foreground mb-1">
                            Bank Account Number
                        </label>
                        <input
                            type="text"
                            id="bank_account_number"
                            value={formData.bank_account_number}
                            onChange={(e) => setFormData(prev => ({ ...prev, bank_account_number: e.target.value }))}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={loading}
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label htmlFor="email" className="block text-sm font-medium text-foreground mb-1">
                                Email
                            </label>
                            <input
                                type="email"
                                id="email"
                                value={formData.email}
                                onChange={(e) => setFormData(prev => ({ ...prev, email: e.target.value }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                        <div>
                            <label htmlFor="phone" className="block text-sm font-medium text-foreground mb-1">
                                Phone
                            </label>
                            <input
                                type="text"
                                id="phone"
                                value={formData.phone}
                                onChange={(e) => setFormData(prev => ({ ...prev, phone: e.target.value }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                    </div>

                    <div>
                        <label htmlFor="notes" className="block text-sm font-medium text-foreground mb-1">
                            Notes
                        </label>
                        <textarea
                            id="notes"
                            value={formData.notes}
                            onChange={(e) => setFormData(prev => ({ ...prev, notes: e.target.value }))}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            rows={2}
                            disabled={loading}
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label htmlFor="billingo_unit" className="block text-sm font-medium text-foreground mb-1">
                                Invoice Unit
                            </label>
                            <input
                                type="text"
                                id="billingo_unit"
                                value={formData.billingo_unit}
                                onChange={(e) => setFormData(prev => ({ ...prev, billingo_unit: e.target.value }))}
                                placeholder="Uses account default if left empty"
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>
                        <div>
                            <label htmlFor="billingo_unit_price_type" className="block text-sm font-medium text-foreground mb-1">
                                Invoice Price Type
                            </label>
                            <select
                                id="billingo_unit_price_type"
                                value={formData.billingo_unit_price_type}
                                onChange={(e) => setFormData(prev => ({ ...prev, billingo_unit_price_type: e.target.value as '' | 'net' | 'gross' }))}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            >
                                <option value="">Account default</option>
                                <option value="net">Net</option>
                                <option value="gross">Gross</option>
                            </select>
                        </div>
                    </div>

                    <div className="flex space-x-3 pt-4">
                        <button
                            type="button"
                            onClick={handleClose}
                            disabled={loading}
                            className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={loading || !formData.name?.trim()}
                            className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                        >
                            {loading ? 'Creating...' : 'Create Client'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}
