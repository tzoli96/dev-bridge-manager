import React from 'react';
import { cn } from '@/lib/utils';

interface LoadingSpinnerProps {
    size?: 'sm' | 'md' | 'lg' | 'xl';
    color?: 'primary' | 'secondary' | 'success' | 'warning' | 'danger' | 'gray';
    fullScreen?: boolean;
    text?: string;
    showText?: boolean;
    className?: string;
}

export default function LoadingSpinner({
    size = 'md',
    color = 'primary',
    fullScreen = true,
    text = 'Loading...',
    showText = true,
    className
}: LoadingSpinnerProps) {
    const sizeClasses = {
        sm: 'h-4 w-4 border-2',
        md: 'h-8 w-8 border-2',
        lg: 'h-12 w-12 border-3',
        xl: 'h-16 w-16 border-4'
    };

    const colorClasses = {
        primary: 'border-blue-600',
        secondary: 'border-gray-600',
        success: 'border-green-600',
        warning: 'border-yellow-600',
        danger: 'border-red-600',
        gray: 'border-gray-400'
    };

    const textSizeClasses = {
        sm: 'text-xs',
        md: 'text-sm',
        lg: 'text-base',
        xl: 'text-lg'
    };

    const spinner = (
        <div className="flex items-center justify-center">
            <div className={cn(
                "animate-spin rounded-full border-b-transparent",
                sizeClasses[size],
                colorClasses[color]
            )}></div>
            {showText && (
                <span className={cn(
                    "ml-3 text-gray-600 animate-pulse",
                    textSizeClasses[size]
                )}>
                    {text}
                </span>
            )}
        </div>
    );

    if (fullScreen) {
        return (
            <div className={cn(
                "min-h-screen bg-gray-50 flex items-center justify-center",
                className
            )}>
                {spinner}
            </div>
        );
    }

    return (
        <div className={className}>
            {spinner}
        </div>
    );
}