/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}'],
  theme: {
    extend: {
      colors: {
        brand: {
          green: '#00c07f',
          'green-light': '#33d49e',
          'green-dark': '#009963',
          dark: '#1a1a2e',
        },
      },
    },
  },
  plugins: [],
};
