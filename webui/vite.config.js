import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';

const backendTarget = process.env.XSQL_WEB_PROXY_TARGET || 'http://127.0.0.1:8788';

export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api/v1': {
        target: backendTarget,
        changeOrigin: true
      }
    }
  }
});
