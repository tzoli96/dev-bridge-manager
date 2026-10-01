CREATE TABLE profit_parameter_sets (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    percent_items JSONB NOT NULL DEFAULT '[]',
    fixed_monthly_costs JSONB NOT NULL DEFAULT '[]',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE profit_scenarios (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    horizon_months INTEGER NOT NULL CHECK (horizon_months BETWEEN 3 AND 6),
    parameter_set_id INTEGER NOT NULL REFERENCES profit_parameter_sets(id) ON DELETE RESTRICT,
    capacity_hours_per_month NUMERIC(6,2) NOT NULL,
    client_adjustments JSONB NOT NULL DEFAULT '[]',
    new_clients JSONB NOT NULL DEFAULT '[]',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_profit_scenarios_parameter_set ON profit_scenarios (parameter_set_id);
