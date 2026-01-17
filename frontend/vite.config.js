import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Determine backend URL - use environment variable or default based on context
const getBackendUrl = () => {
  if (process.env.BACKEND_URL) {
    return process.env.BACKEND_URL
  }
  // Default to localhost for local development
  // In Docker, BACKEND_URL should be set to http://host.docker.internal:8181
  return 'http://localhost:8181'
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3030,
    host: '0.0.0.0', // Allow external connections (needed for Docker)
    proxy: {
      '/api': {
        target: getBackendUrl(),
        changeOrigin: true,
        rewrite: (path) => path,
      },
    },
  },
  preview: {
    port: 3030,
    host: '0.0.0.0', // Allow external connections (needed for Docker)
    proxy: {
      '/api': {
        target: getBackendUrl(),
        changeOrigin: true,
        rewrite: (path) => path,
      },
    },
  },
})
