import React from 'react';
import { cn } from '@/lib/utils';

interface ProgressBarProps {
    percentage: number;
    size?: 'sm' | 'md' | 'lg';
    variant?: 'default' | 'danger' | 'success';
    className?: string;
}

const sizeClasses = {
    sm: 'h-1.5',
    md: 'h-2',
    lg: 'h-3',
};

const variantClasses = {
    default: 'bg-blue-500',
    danger: 'bg-red-500',
    success: 'bg-green-500',
};

export const ProgressBar: React.FC<ProgressBarProps> = ({ percentage, size = 'md', variant = 'default', className }) => {
    const clamped = Math.min(100, Math.max(0, percentage));

    return (
        <div className={cn('w-full bg-gray-200 rounded-full overflow-hidden', sizeClasses[size], className)}>
            <div
                className={cn('h-full rounded-full transition-all duration-300', variantClasses[variant])}
                style={{ width: `${clamped}%` }}
            />
        </div>
    );
};
