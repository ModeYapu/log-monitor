import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    lib: {
      entry: './src/index.ts',
      name: 'LogMonitor',
      formats: ['umd', 'es'],
      fileName: (format) => `logmonitor.${format === 'umd' ? 'min.js' : 'mjs'}`,
    },
    rollupOptions: {
      output: {
        globals: {
          'logmonitor': 'LogMonitor',
        },
      },
    },
    sourcemap: true,
  },
});
