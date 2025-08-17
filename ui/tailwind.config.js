/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        // Tokyo Night Theme
        "tokyo-bg": "var(--tokyo-bg)",
        "tokyo-bg-dark": "var(--tokyo-bg-dark)",
        "tokyo-bg-highlight": "var(--tokyo-bg-highlight)",
        "tokyo-terminal-black": "var(--tokyo-terminal-black)",
        "tokyo-fg": "var(--tokyo-fg)",
        "tokyo-fg-dark": "var(--tokyo-fg-dark)",
        "tokyo-fg-gutter": "var(--tokyo-fg-gutter)",
        "tokyo-dark3": "var(--tokyo-dark3)",
        "tokyo-comment": "var(--tokyo-comment)",
        "tokyo-dark5": "var(--tokyo-dark5)",
        "tokyo-blue0": "var(--tokyo-blue0)",
        "tokyo-blue": "var(--tokyo-blue)",
        "tokyo-cyan": "var(--tokyo-cyan)",
        "tokyo-blue1": "var(--tokyo-blue1)",
        "tokyo-blue2": "var(--tokyo-blue2)",
        "tokyo-blue5": "var(--tokyo-blue5)",
        "tokyo-blue6": "var(--tokyo-blue6)",
        "tokyo-blue7": "var(--tokyo-blue7)",
        "tokyo-purple": "var(--tokyo-purple)",
        "tokyo-magenta": "var(--tokyo-magenta)",
        "tokyo-magenta2": "var(--tokyo-magenta2)",
        "tokyo-red": "var(--tokyo-red)",
        "tokyo-red1": "var(--tokyo-red1)",
        "tokyo-orange": "var(--tokyo-orange)",
        "tokyo-yellow": "var(--tokyo-yellow)",
        "tokyo-green": "var(--tokyo-green)",
        "tokyo-green1": "var(--tokyo-green1)",
        "tokyo-green2": "var(--tokyo-green2)",
        "tokyo-teal": "var(--tokyo-teal)",
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [require("@tailwindcss/typography")],
}