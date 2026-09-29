// frontend/src/services/marketingContactsService.ts
import { apiClient } from '@/lib/api';

export interface MarketingContact {
    id: number;
    email: string;
    first_name: string;
    last_name: string;
    subscribed: boolean;
    tags: string;
    source: string;
    notes: string;
    created_by: number;
    created_at: string;
    updated_at: string;
}

export interface MarketingContactInput {
    email: string;
    first_name: string;
    last_name: string;
    subscribed: boolean;
    tags: string;
    source: string;
    notes: string;
}

export interface MarketingContactFilters {
    search?: string;
    subscribed?: boolean;
    tag?: string;
}

export interface ImportResult {
    created: number;
    updated: number;
    skipped: number;
    errors: string[];
}

export const marketingContactsService = {
    async list(filters: MarketingContactFilters = {}): Promise<MarketingContact[]> {
        return apiClient.get('/marketing-contacts', filters);
    },

    async create(data: MarketingContactInput): Promise<MarketingContact> {
        return apiClient.post('/marketing-contacts', data);
    },

    async update(id: number, data: MarketingContactInput): Promise<MarketingContact> {
        return apiClient.put(`/marketing-contacts/${id}`, data);
    },

    async remove(id: number): Promise<void> {
        return apiClient.delete(`/marketing-contacts/${id}`);
    },

    async importCsv(file: File, overwrite: boolean): Promise<ImportResult> {
        return apiClient.uploadFiles('/marketing-contacts/import', [file], {
            overwrite: String(overwrite),
        });
    },

    async exportCsv(filters: MarketingContactFilters = {}): Promise<Blob> {
        const params = new URLSearchParams();
        if (filters.subscribed !== undefined) params.set('subscribed', String(filters.subscribed));
        if (filters.tag) params.set('tag', filters.tag);
        const query = params.toString();
        return apiClient.getBlob(`/marketing-contacts/export${query ? `?${query}` : ''}`);
    },
};
