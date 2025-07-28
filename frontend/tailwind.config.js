module.exports = {
    content: [
        './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
        './src/components/**/*.{js,ts,jsx,tsx,mdx}',
        './src/app/**/*.{js,ts,jsx,tsx,mdx}',
    ],
    theme: {
        extend: {
            keyframes: {
                shake: {
                    '0%, 100%': { transform: 'translateX(0)' },
                    '10%, 30%, 50%, 70%, 90%': { transform: 'translateX(-4px)' },
                    '20%, 40%, 60%, 80%': { transform: 'translateX(4px)' },
                },
                'bounce-subtle': {
                    '0%, 100%': { 
                        transform: 'translateY(0)',
                        animationTimingFunction: 'cubic-bezier(0.8, 0, 1, 1)'
                    },
                    '50%': { 
                        transform: 'translateY(-5px)',
                        animationTimingFunction: 'cubic-bezier(0, 0, 0.2, 1)'
                    },
                },
                'pulse-slow': {
                    '0%, 100%': { opacity: 1 },
                    '50%': { opacity: 0.7 },
                },
            },
            animation: {
                shake: 'shake 0.8s cubic-bezier(.36,.07,.19,.97) both',
                'bounce-subtle': 'bounce-subtle 2s ease-in-out infinite',
                'pulse-slow': 'pulse-slow 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
            },
            transitionProperty: {
                'height': 'height',
                'spacing': 'margin, padding',
            },
            transitionDuration: {
                '400': '400ms',
            },
        },
    },
    plugins: [],
}