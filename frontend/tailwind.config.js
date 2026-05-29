/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        /* Legacy semantic names (kept for compatibility) */
        'dark-bg':             '#0a0a0a',
        'dark-surface':        '#141414',
        'dark-surface-hover':  '#1a1a1a',
        'dark-border':         '#2a2a3e',
        'dark-text':           '#e0e0e0',
        'dark-text-muted':     '#8b8b9e',
        'dark-text-secondary': '#6b6b7e',

        /* Theme-aware colors via CSS variables */
        'theme-bg':             'var(--color-bg)',
        'theme-surface':        'var(--color-surface)',
        'theme-input':          'var(--color-input)',
        'theme-card':           'var(--color-card)',
        'theme-card-hover':     'var(--color-card-hover)',
        'theme-border':         'var(--color-border)',
        'theme-border-subtle':  'var(--color-border-subtle)',
        'theme-border-focus':   'var(--color-border-focus)',
        'theme-toggle':         'var(--color-toggle)',
        'theme-text':           'var(--color-text)',
        'theme-text-muted':     'var(--color-text-muted)',
        'theme-text-secondary': 'var(--color-text-secondary)',
        'theme-log':            'var(--color-log)',
        'theme-log-hover':      'var(--color-log-hover)',
      },
      fontFamily: {
        mono: ['Courier New', 'monospace'],
      },
    },
  },
  plugins: [],
}
