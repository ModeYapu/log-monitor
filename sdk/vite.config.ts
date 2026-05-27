import { defineConfig } from 'vite';
import { dts } from 'vite-plugin-dts';

export default defineConfig({
  plugins: [
    dts({
      insertTypesEntry: true,
    }),
  ],
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
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: false,
        pure_funcs: [],
      },
    },
    sourcemap: true,
  },
});
