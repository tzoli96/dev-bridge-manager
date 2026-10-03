import { apiClient } from '@/lib/api';

export interface UnbilledProjectRow {
    project_id: number;
    project_name: string;
    pricing_type: string;
    hourly_based: boolean;
    hourly_rate: number | null;
    logged_hours: number;
    billable_hours: number;
    billable_amount: number | null;
    in_progress_hours: number;
    oldest_billable_date: string | null;
    missing_rate: boolean;
}

export interface UnbilledClientGroup {
    client_id: number | null;
    client_name: string;
    unassigned: boolean;
    billable_hours: number;
    billable_amount: number;
    in_progress_hours: number;
    projects: UnbilledProjectRow[];
}

export interface UnbilledHours {
    as_of: string;
    cutoff_date: string;
    totals: {
        billable_hours: number;
        billable_amount: number;
        in_progress_hours: number;
    };
    clients: UnbilledClientGroup[];
    fully_invoiced_projects: number;
    warnings: string[];
}

export const UnbilledHoursService = {
    async get(): Promise<UnbilledHours> {
        return apiClient.get('/billing/unbilled-hours');
    },
};
