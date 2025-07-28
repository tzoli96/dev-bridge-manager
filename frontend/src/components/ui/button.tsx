import React from 'react';
import { cn } from '@/lib/utils';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: 'primary' | 'secondary' | 'ghost' | 'outline' | 'success' | 'danger';
    size?: 'sm' | 'md' | 'lg';
    icon?: React.ReactNode;
    loading?: boolean;
    children?: React.ReactNode;
    ripple?: boolean;
}

export const Button: React.FC<ButtonProps> = ({
                                                  variant = 'primary',
                                                  size = 'md',
                                                  icon,
                                                  loading = false,
                                                  children,
                                                  className,
                                                  disabled,
                                                  ripple = true,
                                                  onClick,
                                                  ...props
                                              }) => {
    // Ref for ripple effect
    const buttonRef = React.useRef<HTMLButtonElement>(null);

    // Handle click with ripple effect
    const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
        if (ripple && !disabled && !loading && buttonRef.current) {
            const button = buttonRef.current;
            const rect = button.getBoundingClientRect();
            const x = e.clientX - rect.left;
            const y = e.clientY - rect.top;
            
            const ripple = document.createElement('span');
            ripple.style.position = 'absolute';
            ripple.style.left = `${x}px`;
            ripple.style.top = `${y}px`;
            ripple.style.transform = 'translate(-50%, -50%) scale(0)';
            ripple.style.width = '0';
            ripple.style.height = '0';
            ripple.style.borderRadius = '50%';
            ripple.style.backgroundColor = 'rgba(255, 255, 255, 0.4)';
            ripple.style.pointerEvents = 'none';
            ripple.style.transition = 'all 0.6s ease-out';
            
            button.appendChild(ripple);
            
            // Trigger animation
            setTimeout(() => {
                const size = Math.max(button.offsetWidth, button.offsetHeight) * 2;
                ripple.style.width = `${size}px`;
                ripple.style.height = `${size}px`;
                ripple.style.transform = 'translate(-50%, -50%) scale(1)';
                ripple.style.opacity = '0';
            }, 10);
            
            // Remove ripple after animation
            setTimeout(() => {
                ripple.remove();
            }, 600);
        }
        
        // Call original onClick handler
        if (onClick) onClick(e);
    };

    const baseClasses = 'inline-flex items-center justify-center gap-2 rounded-lg font-medium relative overflow-hidden transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

    const variantClasses = {
        primary: 'bg-blue-600 text-white hover:bg-blue-700 hover:shadow-md active:shadow-inner active:scale-[0.98] focus:ring-blue-500',
        secondary: 'bg-gray-500 text-white hover:bg-gray-600 hover:shadow-md active:shadow-inner active:scale-[0.98] focus:ring-gray-500',
        ghost: 'text-gray-500 hover:bg-gray-100 active:bg-gray-200 active:scale-[0.98] focus:ring-gray-300',
        outline: 'border border-gray-300 text-gray-700 hover:bg-gray-50 hover:border-gray-400 active:bg-gray-100 active:scale-[0.98] focus:ring-gray-400',
        success: 'bg-green-600 text-white hover:bg-green-700 hover:shadow-md active:shadow-inner active:scale-[0.98] focus:ring-green-500',
        danger: 'bg-red-600 text-white hover:bg-red-700 hover:shadow-md active:shadow-inner active:scale-[0.98] focus:ring-red-500',
    };

    const sizeClasses = {
        sm: 'p-2 text-sm',
        md: 'px-4 py-2 text-sm',
        lg: 'px-6 py-3 text-base'
    };

    return (
        <button
            ref={buttonRef}
            className={cn(
                baseClasses,
                variantClasses[variant],
                sizeClasses[size],
                className
            )}
            disabled={disabled || loading}
            onClick={handleClick}
            {...props}
        >
            {loading ? (
                <div className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
            ) : icon}
            {children}
        </button>
    );
};
