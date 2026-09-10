/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        // Codex 深色主题
        ink: {
          950: '#0a0a0a',
          900: '#111114',
          850: '#16161a',
          800: '#1c1c21',
          750: '#222228',
          700: '#2a2a31',
          600: '#38383f',
          500: '#4b4b55',
          400: '#6b6b76',
          300: '#8b8b96',
        },
        accent: {
          DEFAULT: '#10a37f',
          dim: '#0d8a6c',
        },
      },
      fontSize: {
        xxs: ['10px', '14px'],
      },
      boxShadow: {
        panel: '0 1px 0 0 rgba(0,0,0,0.4), 0 4px 16px rgba(0,0,0,0.5)',
      },
    },
  },
  plugins: [],
}
