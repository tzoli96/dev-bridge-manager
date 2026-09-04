'use client';

import React from 'react';
import { cn } from '@/lib/utils';

export interface TabItem {
    id: string;
    label: string;
}

interface TabsProps {
    tabs: TabItem[];
    activeTab: string;
    onChange: (tabId: string) => void;
}

export const Tabs: React.FC<TabsProps> = ({ tabs, activeTab, onChange }) => {
    return (
        <div className="flex border-b border-gray-200 px-6">
            {tabs.map((tab) => (
                <button
                    key={tab.id}
                    type="button"
                    onClick={() => onChange(tab.id)}
                    className={cn(
                        'px-4 py-3 text-sm font-medium border-b-2 -mb-px transition-colors',
                        activeTab === tab.id
                            ? 'border-blue-600 text-blue-600'
                            : 'border-transparent text-gray-500 hover:text-gray-700'
                    )}
                >
                    {tab.label}
                </button>
            ))}
        </div>
    );
};
