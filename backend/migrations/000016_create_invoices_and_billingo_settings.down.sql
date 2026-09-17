-- backend/migrations/000016_create_invoices_and_billingo_settings.down.sql

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('invoices.create', 'invoices.read', 'billingo_settings.manage')
);
DELETE FROM permissions WHERE name IN ('invoices.create', 'invoices.read', 'billingo_settings.manage');

DROP TABLE IF EXISTS billingo_settings;
DROP TABLE IF EXISTS invoices;

ALTER TABLE clients DROP COLUMN billingo_partner_id;
