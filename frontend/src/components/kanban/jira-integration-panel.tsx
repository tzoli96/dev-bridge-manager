'use client';

import React from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAuth } from '@/hooks/auth/use-auth';
import { isSuperAdmin } from '@/utils/permissions';
import { jiraIntegrationService } from '@/services/kanban/jiraIntegrationService';
import type { JiraIntegrationStatus } from '@/types/kanban';

interface JiraIntegrationPanelProps {
    projectId: string;
    boardId: string;
}

export const JiraIntegrationPanel: React.FC<JiraIntegrationPanelProps> = ({ projectId, boardId }) => {
    const { user } = useAuth();
    const [status, setStatus] = React.useState<JiraIntegrationStatus | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [saving, setSaving] = React.useState(false);
    const [baseUrl, setBaseUrl] = React.useState('');
    const [email, setEmail] = React.useState('');
    const [apiToken, setApiToken] = React.useState('');
    const [projectKey, setProjectKey] = React.useState('');

    const loadStatus = React.useCallback(() => {
        setLoading(true);
        jiraIntegrationService.getStatus(projectId, boardId)
            .then(setStatus)
            .catch(() => setStatus({ connected: false }))
            .finally(() => setLoading(false));
    }, [projectId, boardId]);

    React.useEffect(() => {
        loadStatus();
    }, [loadStatus]);

    if (!isSuperAdmin(user)) return null;
    if (loading) return null;

    const handleConnect = async () => {
        setError(null);
        setSaving(true);
        try {
            await jiraIntegrationService.connect(projectId, boardId, { baseUrl, email, apiToken, projectKey });
            setBaseUrl('');
            setEmail('');
            setApiToken('');
            setProjectKey('');
            loadStatus();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to connect to Jira');
        } finally {
            setSaving(false);
        }
    };

    const handleDisconnect = async () => {
        if (!window.confirm('Disconnect the Jira integration? Already-mirrored tasks and columns will stay in place but will no longer be synced.')) return;
        setError(null);
        setSaving(true);
        try {
            await jiraIntegrationService.disconnect(projectId, boardId);
            loadStatus();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to disconnect the Jira integration');
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="border border-dashed border-input rounded-lg p-3 space-y-3">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Jira Integration</h3>

            {error && (
                <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            {status?.connected ? (
                <div className="space-y-2">
                    <p className="text-sm text-foreground">
                        Connected: <span className="font-medium">{status.projectKey}</span> ({status.email})
                    </p>
                    <p className="text-xs text-muted-foreground">
                        {status.lastSyncAt ? `Last synced: ${new Date(status.lastSyncAt).toLocaleString()}` : 'Not synced yet'}
                    </p>
                    {status.lastSyncError && (
                        <p className="text-xs text-destructive">Last sync error: {status.lastSyncError}</p>
                    )}
                    <Button variant="outline" size="sm" loading={saving} onClick={handleDisconnect}>
                        Disconnect
                    </Button>
                </div>
            ) : (
                <div className="space-y-2">
                    <Input label="Base URL" value={baseUrl} onChange={setBaseUrl} placeholder="https://yourcompany.atlassian.net" />
                    <Input label="Email" value={email} onChange={setEmail} placeholder="me@example.com" />
                    <Input label="API Token" type="password" value={apiToken} onChange={setApiToken} />
                    <Input label="Project Key" value={projectKey} onChange={setProjectKey} placeholder="PROJ" />
                    <Button
                        size="sm"
                        loading={saving}
                        disabled={!baseUrl || !email || !apiToken || !projectKey}
                        onClick={handleConnect}
                    >
                        Connect
                    </Button>
                </div>
            )}
        </div>
    );
};
