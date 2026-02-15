import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Determine backend URL - use environment variable or default based on context
const getBackendUrl = () => {
  if (process.env.BACKEND_URL) {
    return process.env.BACKEND_URL
  }
  // Use 127.0.0.1 (IPv4) so proxy doesn't try ::1 (IPv6) and get ECONNREFUSED on some servers
  // In Docker, set BACKEND_URL=http://host.docker.internal:8181
  return 'http://127.0.0.1:8181'
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3030,
    host: 'localhost', // Local development
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
    host: '0.0.0.0', // Allow external connections
    allowedHosts: [
      'crypto-alert.log.skyproton.com',
    ],
    proxy: {
      '/api': {
        target: getBackendUrl(),
        changeOrigin: true,
        rewrite: (path) => path,
      },
    },
  },
})
