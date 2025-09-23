import { Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/contexts/ThemeContext'

export function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={toggleTheme}
      className="relative w-9 h-9"
      aria-label="Toggle theme"
    >
      <Sun className={`h-4 w-4 transition-all ${
        theme === 'light' ? 'rotate-0 scale-100' : 'rotate-90 scale-0'
      }`} />
      <Moon className={`absolute h-4 w-4 transition-all ${
        theme === 'dark' ? 'rotate-0 scale-100' : '-rotate-90 scale-0'
      }`} />
    </Button>
  )
}