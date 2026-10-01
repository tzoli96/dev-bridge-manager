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

export type PercentBase = 'revenue' | 'after_costs';

export interface PercentItem {
    label: string;
    percent: number;
    base: PercentBase;
}

export interface FixedCost {
    label: string;
    amount: number;
}

export interface ParameterSet {
    id: number;
    name: string;
    percent_items: PercentItem[];
    fixed_monthly_costs: FixedCost[];
    created_by: number;
    created_at: string;
    updated_at: string;
}

export interface ParameterSetInput {
    name: string;
    percent_items: PercentItem[];
    fixed_monthly_costs: FixedCost[];
}

export interface ScenarioAdjustment {
    client_id: number;
    new_hourly_rate: number | null;
    new_fixed_price: number | null;
    hours_delta: number;
    drops: boolean;
}

export interface ScenarioNewClient {
    name: string;
    monthly_revenue: number;
    monthly_hours: number;
}

export interface ScenarioInput {
    name: string;
    horizon_months: number;
    parameter_set_id: number;
    capacity_hours_per_month: number;
    client_adjustments: ScenarioAdjustment[];
    new_clients: ScenarioNewClient[];
}

export interface Scenario extends ScenarioInput {
    id: number;
    created_by: number;
    created_at: string;
    updated_at: string;
}

export interface ScenarioItemAmount {
    label: string;
    percent: number;
    base: PercentBase;
    amount: number;
}

export interface ScenarioMonth {
    revenue: number;
    fixed_costs: number;
    revenue_items: ScenarioItemAmount[];
    result_after_costs: number;
    after_costs_items: ScenarioItemAmount[];
    net_profit: number;
    required_hours: number;
    capacity_hours: number;
    utilization: number | null;
    overloaded: boolean;
    net_profit_per_capacity_hour: number | null;
}

export interface ScenarioClientResult {
    client_id: number;
    name: string;
    is_new: boolean;
    dropped: boolean;
    base_revenue: number;
    base_hours: number;
    revenue: number;
    hours: number;
}

export interface ScenarioResult {
    months: string[];
    monthly: ScenarioMonth;
    baseline: ScenarioMonth;
    horizon: { revenue: number; net_profit: number };
    baseline_horizon: { revenue: number; net_profit: number };
    clients: ScenarioClientResult[];
    low_data: boolean;
    warnings: string[];
    basis: {
        parameter_set_name: string;
        percent_items: PercentItem[];
        fixed_monthly_costs: FixedCost[];
        capacity_hours: number;
        baseline_months: string[];
        horizon_months: number;
    };
}

export const profitabilityService = {
    async overview(months: number): Promise<ProfitabilityOverview> {
        return apiClient.get(`/profitability/overview?months=${months}`);
    },

    async forecast(months: number): Promise<ProfitabilityForecast> {
        return apiClient.get(`/profitability/forecast?months=${months}`);
    },

    async listParameterSets(): Promise<ParameterSet[]> {
        return apiClient.get('/profitability/parameter-sets');
    },

    async createParameterSet(data: ParameterSetInput): Promise<ParameterSet> {
        return apiClient.post('/profitability/parameter-sets', data);
    },

    async updateParameterSet(id: number, data: ParameterSetInput): Promise<ParameterSet> {
        return apiClient.put(`/profitability/parameter-sets/${id}`, data);
    },

    async deleteParameterSet(id: number): Promise<void> {
        return apiClient.delete(`/profitability/parameter-sets/${id}`);
    },

    async listScenarios(): Promise<Scenario[]> {
        return apiClient.get('/profitability/scenarios');
    },

    async createScenario(data: ScenarioInput): Promise<Scenario> {
        return apiClient.post('/profitability/scenarios', data);
    },

    async updateScenario(id: number, data: ScenarioInput): Promise<Scenario> {
        return apiClient.put(`/profitability/scenarios/${id}`, data);
    },

    async deleteScenario(id: number): Promise<void> {
        return apiClient.delete(`/profitability/scenarios/${id}`);
    },

    async computeScenario(data: ScenarioInput): Promise<ScenarioResult> {
        return apiClient.post('/profitability/scenarios/compute', data);
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
