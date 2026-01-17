/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'dark-bg': '#0a0a0a',
        'dark-surface': '#141414',
        'dark-surface-hover': '#1a1a1a',
        'dark-border': '#2a2a3e',
        'dark-text': '#e0e0e0',
        'dark-text-muted': '#8b8b9e',
        'dark-text-secondary': '#6b6b7e',
      },
      fontFamily: {
        mono: ['Courier New', 'monospace'],
      },
    },
  },
  plugins: [],
}
