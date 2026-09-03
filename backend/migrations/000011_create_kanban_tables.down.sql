-- 000011_create_kanban_tables.down.sql

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN (
        'tasks.create', 'tasks.update', 'tasks.delete', 'tasks.move',
        'kanban.manage_columns', 'time_tracking.view', 'time_tracking.edit'
    )
);

DELETE FROM permissions WHERE name IN (
    'tasks.create', 'tasks.update', 'tasks.delete', 'tasks.move',
    'kanban.manage_columns', 'time_tracking.view', 'time_tracking.edit'
);

DROP TABLE IF EXISTS task_time_entries;
DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS kanban_columns;
