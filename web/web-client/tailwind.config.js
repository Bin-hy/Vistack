import uiConfig from '../ui/tailwind.config.js';

/** @type {import('tailwindcss').Config} */
export default {
  presets: [uiConfig],
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
    '../ui/src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        glass: 'hsl(var(--glass) / <alpha-value>)',
      },
      boxShadow: {
        glow: '0 0 0 1px hsl(var(--primary) / 0.25), 0 8px 32px -12px hsl(var(--primary) / 0.55)',
        'glow-sm': '0 0 20px -6px hsl(var(--primary) / 0.55)',
        soft: '0 8px 30px -12px rgb(0 0 0 / 0.6)',
      },
      keyframes: {
        glow: {
          '0%, 100%': { opacity: '0.5' },
          '50%': { opacity: '1' },
        },
        'fade-in': {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        'fade-up': {
          from: { opacity: '0', transform: 'translateY(10px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'scale-in': {
          from: { opacity: '0', transform: 'scale(0.96)' },
          to: { opacity: '1', transform: 'scale(1)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
      },
      animation: {
        glow: 'glow 8s ease-in-out infinite',
        'fade-in': 'fade-in 0.25s ease-out',
        'fade-up': 'fade-up 0.4s cubic-bezier(0.16, 1, 0.3, 1)',
        'scale-in': 'scale-in 0.15s ease-out',
        shimmer: 'shimmer 1.8s ease-in-out infinite',
      },
    },
  },
  plugins: [],
}
