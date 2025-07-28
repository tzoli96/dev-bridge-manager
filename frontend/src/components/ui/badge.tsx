import React, { useEffect, useState } from 'react';
import { cn } from '@/lib/utils';

interface BadgeProps {
    children: React.ReactNode;
    variant?: 'default' | 'secondary' | 'success' | 'warning' | 'danger';
    size?: 'sm' | 'md' | 'lg';
    className?: string;
    style?: React.CSSProperties;
    pulse?: boolean;
    dot?: boolean;
    animated?: boolean;
}

export const Badge: React.FC<BadgeProps> = ({
    children,
    variant = 'default',
    size = 'sm',
    className,
    style,
    pulse = false,
    dot = false,
    animated = true
}) => {
    // State for fade-in animation
    const [isVisible, setIsVisible] = useState(!animated);
    
    // Effect to trigger fade-in animation
    useEffect(() => {
        if (animated) {
            const timer = setTimeout(() => {
                setIsVisible(true);
            }, 50);
            return () => clearTimeout(timer);
        }
    }, [animated]);

    const variantClasses = {
        default: 'bg-gray-100 text-gray-800',
        secondary: 'bg-blue-100 text-blue-800',
        success: 'bg-green-100 text-green-800',
        warning: 'bg-yellow-100 text-yellow-800',
        danger: 'bg-red-100 text-red-800'
    };

    const sizeClasses = {
        sm: 'px-2 py-1 text-xs',
        md: 'px-3 py-1 text-sm',
        lg: 'px-4 py-2 text-base'
    };
    
    // Dot indicator colors
    const dotColors = {
        default: 'bg-gray-500',
        secondary: 'bg-blue-500',
        success: 'bg-green-500',
        warning: 'bg-yellow-500',
        danger: 'bg-red-500'
    };

    return (
        <span
            className={cn(
                'inline-flex items-center rounded-full font-medium transition-all duration-200',
                variantClasses[variant],
                sizeClasses[size],
                pulse && 'animate-pulse',
                animated && 'transform transition-opacity duration-300',
                animated && (isVisible ? 'opacity-100' : 'opacity-0'),
                className
            )}
            style={{
                ...style,
                transform: animated && !isVisible ? 'scale(0.95)' : 'scale(1)'
            }}
        >
            {dot && (
                <span 
                    className={cn(
                        'inline-block w-2 h-2 rounded-full mr-1',
                        dotColors[variant]
                    )}
                />
            )}
            {children}
        </span>
    );
};