import { Sun, Moon } from "lucide-react";
import { useTheme } from "@/hooks/use-theme";
import { Button } from "@/components/ui/button";

export function ThemeToggle() {
  const { theme, toggleTheme } = useTheme();

  return (
    <Button variant="ghost" size="icon" className="h-8 w-8" onClick={toggleTheme} title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}>
      {theme === "dark" ? <Sun className="h-4 w-4 text-muted-foreground" /> : <Moon className="h-4 w-4 text-muted-foreground" />}
    </Button>
  );
}
