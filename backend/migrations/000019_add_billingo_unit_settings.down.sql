ALTER TABLE clients
    DROP COLUMN billingo_unit,
    DROP COLUMN billingo_unit_price_type;

ALTER TABLE billingo_settings
    DROP COLUMN default_unit,
    DROP COLUMN default_unit_price_type;
