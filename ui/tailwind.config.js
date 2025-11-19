/** @type {import('tailwindcss').Config} */
export default {
    content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
  	extend: {
  		colors: {
  			border: 'hsl(var(--border))',
  			input: 'hsl(var(--input))',
  			ring: 'hsl(var(--ring))',
  			background: 'hsl(var(--background))',
  			foreground: 'hsl(var(--foreground))',
  			primary: {
  				DEFAULT: 'hsl(var(--primary))',
  				foreground: 'hsl(var(--primary-foreground))'
  			},
  			secondary: {
  				DEFAULT: 'hsl(var(--secondary))',
  				foreground: 'hsl(var(--secondary-foreground))'
  			},
  			destructive: {
  				DEFAULT: 'hsl(var(--destructive))',
  				foreground: 'hsl(var(--destructive-foreground))'
  			},
  			muted: {
  				DEFAULT: 'hsl(var(--muted))',
  				foreground: 'hsl(var(--muted-foreground))'
  			},
  			accent: {
  				DEFAULT: 'hsl(var(--accent))',
  				foreground: 'hsl(var(--accent-foreground))'
  			},
  			popover: {
  				DEFAULT: 'hsl(var(--popover))',
  				foreground: 'hsl(var(--popover-foreground))'
  			},
  			card: {
  				DEFAULT: 'hsl(var(--card))',
  				foreground: 'hsl(var(--card-foreground))'
  			},
  			// Station color palette - OpenAI inspired soft accents
  			station: {
  				'blue': '#0084FF',
  				'blue-light': '#E6F2FF',
  				'blue-dark': '#0066CC',
  				'lavender': {
  					50: '#F5F3FF',   // Very light lavender for backgrounds
  					100: '#EDE9FE',  // Light lavender
  					200: '#DDD6FE',  // Soft lavender
  					300: '#C4B5FD',  // Medium lavender
  					400: '#A78BFA',  // Vibrant lavender
  					500: '#8B5CF6',  // Deep lavender
  				},
  				'mint': {
  					50: '#F0FDF4',   // Very light mint
  					100: '#DCFCE7',  // Light mint
  					200: '#BBF7D0',  // Soft mint
  					300: '#86EFAC',  // Medium mint
  					400: '#4ADE80',  // Vibrant mint
  					500: '#22C55E',  // Deep mint
  				},
  				'yellow': {
  					50: '#FFFBEB',   // Very light yellow
  					100: '#FEF3C7',  // Light yellow
  					200: '#FDE68A',  // Soft yellow for notes
  					300: '#FCD34D',  // Medium yellow
  				},
  				'gray': {
  					50: '#F8FAFB',
  					100: '#EDF2F6',
  					200: '#DAE5ED',
  					300: '#C4D4DF',
  					400: '#9CA3AF',
  					500: '#6B7280',
  					600: '#4B5563',
  					700: '#374151',
  					800: '#1F2937',
  					900: '#171923'
  				}
  			},
  			chart: {
  				'1': 'hsl(var(--chart-1))',
  				'2': 'hsl(var(--chart-2))',
  				'3': 'hsl(var(--chart-3))',
  				'4': 'hsl(var(--chart-4))',
  				'5': 'hsl(var(--chart-5))'
  			}
  		},
  		borderRadius: {
  			lg: 'var(--radius)',
  			md: 'calc(var(--radius) - 2px)',
  			sm: 'calc(var(--radius) - 4px)'
  		}
  	}
  },
  plugins: [require("@tailwindcss/typography"), require("tailwindcss-animate")],
}