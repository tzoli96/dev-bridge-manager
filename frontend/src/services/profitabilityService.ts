// frontend/src/services/profitabilityService.ts
import { apiClient } from '@/lib/api';

export interface RateRow {
    id: number;
    name: string;
    revenue: number;
    logged_hours: number;
    email_hours: number;
    meeting_hours: number;
    meeting_hours_per_month: number;
    nominal_rate: number | null;
    real_rate: number | null;
    ratio: number | null;
    underpriced_candidate: boolean;
    months_with_data: number;
    low_data: boolean;
}

export interface ProfitSettings {
    minutes_per_inbound_email: number;
    minutes_per_outbound_email: number;
    default_capacity_hours_per_month: number;
    underpriced_ratio_threshold: number;
    updated_at: string;
}

export type ProfitSettingsInput = Omit<ProfitSettings, 'updated_at'>;

export interface ProfitabilityOverview {
    months: string[];
    clients: RateRow[];
    projects: RateRow[];
    warnings: string[];
    settings: ProfitSettings;
}

export interface ForecastClient {
    client_id: number;
    name: string;
    monthly_average: number;
    contract_end_month: string | null;
    months_with_data: number;
    committed: number[];
    dependent: number[];
}

export interface ProfitabilityForecast {
    baseline_months: string[];
    months: string[];
    committed: number[];
    dependent: number[];
    clients: ForecastClient[];
    history_months: number;
    low_data: boolean;
    excluded_fixed_revenue: number;
    warnings: string[];
}

export const profitabilityService = {
    async overview(months: number): Promise<ProfitabilityOverview> {
        return apiClient.get(`/profitability/overview?months=${months}`);
    },

    async forecast(months: number): Promise<ProfitabilityForecast> {
        return apiClient.get(`/profitability/forecast?months=${months}`);
    },

    async getSettings(): Promise<ProfitSettings> {
        return apiClient.get('/profitability/settings');
    },

    async updateSettings(data: ProfitSettingsInput): Promise<ProfitSettings> {
        return apiClient.put('/profitability/settings', data);
    },

    async setMeetingAllowance(clientId: number, hoursPerMonth: number): Promise<void> {
        return apiClient.put(`/profitability/clients/${clientId}/meeting-allowance`, {
            hours_per_month: hoursPerMonth,
        });
    },
};
