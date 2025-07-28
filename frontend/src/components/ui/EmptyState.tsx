import React, { useState, useEffect } from 'react';
import { cn } from '@/lib/utils';
import { Button } from './button';

interface EmptyStateProps {
    icon: 'users' | 'projects' | 'tasks' | 'files' | 'search';
    title: string;
    description: string;
    action?: {
        label: string;
        onClick: () => void;
        variant?: 'primary' | 'secondary' | 'outline';
    };
    className?: string;
    animate?: boolean;
    iconColor?: 'blue' | 'gray' | 'green' | 'purple' | 'yellow';
}

export default function EmptyState({ 
    icon, 
    title, 
    description, 
    action,
    className,
    animate = true,
    iconColor = 'blue'
}: EmptyStateProps) {
    const [isVisible, setIsVisible] = useState(!animate);
    const [isIconVisible, setIsIconVisible] = useState(!animate);
    
    useEffect(() => {
        if (animate) {
            // Stagger the animations for a nicer effect
            const timer = setTimeout(() => {
                setIsVisible(true);
                
                // Delay the icon animation slightly
                setTimeout(() => {
                    setIsIconVisible(true);
                }, 200);
            }, 100);
            
            return () => clearTimeout(timer);
        }
    }, [animate]);
    
    const IconComponent = getIconComponent(icon);
    const iconColorClass = getIconColorClass(iconColor);

    return (
        <div 
            className={cn(
                "bg-white shadow-sm hover:shadow-md transition-all duration-300 rounded-lg p-8 text-center",
                animate && "transition-all duration-500",
                animate && (isVisible ? "opacity-100 transform translate-y-0" : "opacity-0 transform translate-y-8"),
                className
            )}
        >
            <div 
                className={cn(
                    "text-gray-500 mb-6",
                    animate && "transition-all duration-500 transform",
                    animate && (isIconVisible 
                        ? "opacity-100 scale-100" 
                        : "opacity-0 scale-75")
                )}
            >
                <IconComponent className={cn("mx-auto h-16 w-16", iconColorClass)} />
            </div>
            <h3 className="text-xl font-medium text-gray-900 mb-3">{title}</h3>
            <p className="text-gray-500 mb-6 max-w-md mx-auto">{description}</p>
            {action && (
                <Button
                    variant={action.variant || 'primary'}
                    onClick={action.onClick}
                    className={cn(
                        "mt-2",
                        animate && isVisible && "animate-bounce-subtle"
                    )}
                >
                    {action.label}
                </Button>
            )}
        </div>
    );
}

function getIconComponent(icon: string) {
    switch (icon) {
        case 'users':
            return UsersIcon;
        case 'projects':
            return ProjectsIcon;
        case 'tasks':
            return TasksIcon;
        case 'files':
            return FilesIcon;
        case 'search':
            return SearchIcon;
        default:
            return ProjectsIcon;
    }
}

function getIconColorClass(color: string) {
    switch (color) {
        case 'blue':
            return 'text-blue-500';
        case 'gray':
            return 'text-gray-500';
        case 'green':
            return 'text-green-500';
        case 'purple':
            return 'text-purple-500';
        case 'yellow':
            return 'text-yellow-500';
        default:
            return 'text-blue-500';
    }
}

function UsersIcon({ className }: { className?: string }) {
    return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
        </svg>
    );
}

function ProjectsIcon({ className }: { className?: string }) {
    return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
    );
}

function TasksIcon({ className }: { className?: string }) {
    return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
        </svg>
    );
}

function FilesIcon({ className }: { className?: string }) {
    return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
        </svg>
    );
}

function SearchIcon({ className }: { className?: string }) {
    return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
    );
}