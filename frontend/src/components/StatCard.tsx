import { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface StatCardProps {
  label: string;
  value: number | string;
  icon: LucideIcon;
  variant?: "default" | "safe" | "warning" | "critical";
}

const variantStyles = {
  default: "border-border",
  safe: "border-status-safe/30",
  warning: "border-status-warning/30",
  critical: "border-status-critical/30",
};

const iconStyles = {
  default: "text-primary",
  safe: "text-status-safe",
  warning: "text-status-warning",
  critical: "text-status-critical",
};

export function StatCard({ label, value, icon: Icon, variant = "default" }: StatCardProps) {
  return (
    <div className={cn("bg-card border rounded-md p-4 flex items-center gap-3", variantStyles[variant])}>
      <div className={cn("p-2 rounded bg-muted", iconStyles[variant])}>
        <Icon className="h-4 w-4" />
      </div>
      <div>
        <p className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">{label}</p>
        <p className="text-xl font-mono-data font-bold text-foreground">{value}</p>
      </div>
    </div>
  );
}
