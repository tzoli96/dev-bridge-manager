import React from 'react';
import { cn } from '@/lib/utils';

interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'value'> {
    label?: string;
    value: string | number;
    onChange: (value: string) => void;
}

export const Input: React.FC<InputProps> = ({ label, value, onChange, className, id, ...props }) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-');

    return (
        <div className="space-y-1">
            {label && (
                <label htmlFor={inputId} className="block text-sm font-medium text-gray-700">
                    {label}
                </label>
            )}
            <input
                id={inputId}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                className={cn(
                    'block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm',
                    'focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
                    className
                )}
                {...props}
            />
        </div>
    );
};
