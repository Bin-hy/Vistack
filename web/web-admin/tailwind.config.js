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
    extend: {},
  },
  plugins: [],
}
