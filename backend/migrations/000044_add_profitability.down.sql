DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE name IN ('profitability.read', 'profitability.manage'));
DELETE FROM permissions WHERE name IN ('profitability.read', 'profitability.manage');
DROP TABLE IF EXISTS client_meeting_allowances;
DROP TABLE IF EXISTS profit_settings;
