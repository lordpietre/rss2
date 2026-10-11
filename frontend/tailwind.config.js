/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Override default Tailwind colors to use dark theme
        primary: {
          50: '#72FF8A',
          100: '#5CEF73',
          200: '#4AE262',
          300: '#36C96B',
          400: '#28B058',
          500: '#244C32',
          600: '#1A3D27',
          700: '#152D1D',
          800: '#0F1F14',
          900: '#080B0A',
        },
        gray: {
          50: '#DCE8DE',
          100: '#C8D4CA',
          200: '#A8B8AA',
          300: '#829487',
          400: '#5C6B5E',
          500: '#3D4A3F',
          600: '#26362B',
          700: '#1A241C',
          800: '#101512',
          900: '#080B0A',
        },
        rss: {
          bg: {
            main: '#080B0A',
            panel: '#101512',
            secondary: '#151C17',
          },
          green: {
            phosphor: '#72FF8A',
            bright: '#36C96B',
            dim: '#244C32',
          },
          text: {
            primary: '#DCE8DE',
            secondary: '#829487',
          },
          border: '#26362B',
          amber: '#FFC66D',
          red: '#FF5F64',
          blue: '#78B9FF',
        }
      },
      backgroundColor: {
        DEFAULT: '#080B0A',
        primary: '#101512',
        secondary: '#151C17',
      },
      textColor: {
        DEFAULT: '#DCE8DE',
        secondary: '#829487',
      },
      borderColor: {
        DEFAULT: '#26362B',
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'],
        sans: ['Inter', 'system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
      },
      animation: {
        'pulse-green': 'pulse-green 2s ease-in-out infinite',
        'scan': 'scan 8s linear infinite',
      },
      keyframes: {
        'pulse-green': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.7' },
        },
        'scan': {
          '0%': { transform: 'translateY(-100%)' },
          '100%': { transform: 'translateY(100%)' },
        },
      },
    },
  },
  plugins: [],
}
