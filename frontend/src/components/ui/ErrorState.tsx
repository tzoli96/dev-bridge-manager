import React, { useState, useEffect } from 'react';
import { cn } from '@/lib/utils';
import { Button } from './button';

interface ErrorStateProps {
    error: string;
    onRetry?: () => void;
    className?: string;
    animate?: boolean;
    shake?: boolean;
    icon?: boolean;
}

export default function ErrorState({ 
    error, 
    onRetry, 
    className,
    animate = true,
    shake = true,
    icon = true
}: ErrorStateProps) {
    const [isVisible, setIsVisible] = useState(!animate);
    const [shouldShake, setShouldShake] = useState(false);
    
    useEffect(() => {
        if (animate) {
            const timer = setTimeout(() => {
                setIsVisible(true);
                
                if (shake) {
                    // Add a small delay before shaking
                    setTimeout(() => {
                        setShouldShake(true);
                        
                        // Remove shake class after animation completes
                        setTimeout(() => {
                            setShouldShake(false);
                        }, 820); // Shake animation duration + a little buffer
                    }, 100);
                }
            }, 100);
            
            return () => clearTimeout(timer);
        }
    }, [animate, shake]);
    
    return (
        <div 
            className={cn(
                "space-y-4",
                animate && "transition-all duration-300",
                animate && (isVisible ? "opacity-100 transform translate-y-0" : "opacity-0 transform -translate-y-4"),
                className
            )}
        >
            <div 
                className={cn(
                    "bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-lg shadow-sm",
                    "transition-all duration-200 hover:shadow-md",
                    shouldShake && "animate-shake"
                )}
            >
                <div className="flex items-start">
                    {icon && (
                        <div className="mr-3 mt-0.5">
                            <svg className="h-5 w-5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                        </div>
                    )}
                    <div>
                        <div className="font-medium mb-1">Error occurred:</div>
                        <div className="text-red-700">{error}</div>
                    </div>
                </div>
            </div>
            
            {onRetry && (
                <div className="flex justify-center">
                    <Button
                        onClick={onRetry}
                        variant="primary"
                        className="animate-pulse-slow"
                    >
                        Try Again
                    </Button>
                </div>
            )}
        </div>
    );
}