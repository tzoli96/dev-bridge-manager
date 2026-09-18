ALTER TABLE billingo_settings
    ADD COLUMN default_unit VARCHAR(50) NOT NULL DEFAULT 'db',
    ADD COLUMN default_unit_price_type VARCHAR(10) NOT NULL DEFAULT 'net';

ALTER TABLE clients
    ADD COLUMN billingo_unit VARCHAR(50),
    ADD COLUMN billingo_unit_price_type VARCHAR(10);
