import React from 'react';
import { cn } from '@/lib/utils';
import LoadingSpinner from './LoadingSpinner';

interface LoadingStateProps {
    message?: string;
    size?: 'sm' | 'md' | 'lg' | 'xl';
    color?: 'primary' | 'secondary' | 'success' | 'warning' | 'danger' | 'gray';
    showText?: boolean;
    className?: string;
    centered?: boolean;
    fadeIn?: boolean;
}

export default function LoadingState({ 
    message = "Loading...", 
    size = 'md', 
    color = 'primary',
    showText = true,
    className,
    centered = true,
    fadeIn = true
}: LoadingStateProps) {
    const [isVisible, setIsVisible] = React.useState(!fadeIn);
    
    React.useEffect(() => {
        if (fadeIn) {
            const timer = setTimeout(() => {
                setIsVisible(true);
            }, 100);
            return () => clearTimeout(timer);
        }
    }, [fadeIn]);

    return (
        <div 
            className={cn(
                "py-8",
                centered && "flex justify-center items-center",
                fadeIn && "transition-opacity duration-500",
                fadeIn && (isVisible ? "opacity-100" : "opacity-0"),
                className
            )}
        >
            <LoadingSpinner 
                size={size}
                color={color}
                text={message}
                showText={showText}
                fullScreen={false}
            />
        </div>
    );
}