import React from 'react';
import { cn } from '@/lib/utils';

interface SelectOption {
    value: string;
    label: string;
}

interface SelectProps {
    label?: string;
    value: string;
    onChange: (value: string) => void;
    options: SelectOption[];
    className?: string;
}

export const Select: React.FC<SelectProps> = ({ label, value, onChange, options, className }) => {
    const selectId = label?.toLowerCase().replace(/\s+/g, '-');

    return (
        <div className="space-y-1">
            {label && (
                <label htmlFor={selectId} className="block text-sm font-medium text-gray-700">
                    {label}
                </label>
            )}
            <select
                id={selectId}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                className={cn(
                    'block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm bg-white',
                    'focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
                    className
                )}
            >
                {options.map(option => (
                    <option key={option.value} value={option.value}>
                        {option.label}
                    </option>
                ))}
            </select>
        </div>
    );
};
