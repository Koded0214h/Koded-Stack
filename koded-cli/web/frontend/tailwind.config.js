/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        primary: "#0df259",
        "primary-dark": "#0ab845",
        "background-light": "#f5f8f6",
        "background-dark": "#102216",
        "surface-dark": "#142c1c",
        "border-dark": "#232f48",
        "text-muted": "#90cba4",
      },
      fontFamily: {
        display: ["Space Grotesk", "sans-serif"],
        body: ["Noto Sans", "sans-serif"],
        mono: ["monospace"],
      },
      animation: {
        'gradient': 'gradient 3s linear infinite',
        'blink': 'blink 1s step-end infinite',
      },
      keyframes: {
        gradient: {
          '0%, 100%': { backgroundPosition: '0% 50%' },
          '50%': { backgroundPosition: '100% 50%' },
        },
        blink: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0' },
        }
      },
      boxShadow: {
        'neon': '0 0 20px -5px rgba(13, 242, 89, 0.3)',
        'neon-strong': '0 0 30px -5px rgba(13, 242, 89, 0.5)',
      },
      backgroundImage: {
        'grid-pattern': 'linear-gradient(to right, #102316 1px, transparent 1px), linear-gradient(to bottom, #102316 1px, transparent 1px)',
      }
    },
  },
  plugins: [],
}