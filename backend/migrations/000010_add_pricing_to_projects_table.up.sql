-- 000010_add_pricing_to_projects_table.up.sql
-- Óradíjas vagy fix díjas megállapodás a projekthez (HUF)
ALTER TABLE projects ADD COLUMN pricing_type VARCHAR(20); -- 'hourly' | 'fixed'
ALTER TABLE projects ADD COLUMN hourly_rate NUMERIC(12, 2);
ALTER TABLE projects ADD COLUMN fixed_price NUMERIC(12, 2);
