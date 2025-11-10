/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './web/**/*.html',
    './web/**/*.js',
    './src/**/*.ts',
  ],
  theme: {
    extend: {
      colors: {
        primary: '#0066cc',
        secondary: '#6b7280',
        success: '#10b981',
        warning: '#f59e0b',
        error: '#ef4444',
      },
    },
  },
  plugins: [],
}
