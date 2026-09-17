DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE name = 'gmail.manage');
DELETE FROM permissions WHERE name = 'gmail.manage';

DROP TABLE IF EXISTS invoice_notices;
DROP TABLE IF EXISTS emails;
DROP TABLE IF EXISTS gmail_accounts;
