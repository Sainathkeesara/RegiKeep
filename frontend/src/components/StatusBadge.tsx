import { ImageStatus } from "@/lib/api";
import { cn } from "@/lib/utils";

interface StatusBadgeProps {
  status: ImageStatus;
  expiresIn: number;
}

export function StatusBadge({ status, expiresIn }: StatusBadgeProps) {
  const config: Record<ImageStatus, { label: string; className: string }> = {
    safe: { label: "SAFE", className: "bg-status-safe/15 text-status-safe border-status-safe/30" },
    warning: { label: `${expiresIn}d LEFT`, className: "bg-status-warning/15 text-status-warning border-status-warning/30" },
    critical: { label: `${expiresIn}d LEFT`, className: "bg-status-critical/15 text-status-critical border-status-critical/30 animate-pulse-critical" },
    unpinned: { label: "UNPINNED", className: "bg-status-unpinned/15 text-status-unpinned border-status-unpinned/30" },
  };

  const c = config[status] ?? { label: status?.toUpperCase() ?? "UNKNOWN", className: "bg-muted text-muted-foreground border-border" };

  return (
    <span className={cn("inline-flex items-center px-2 py-0.5 rounded text-[10px] font-mono-data font-semibold uppercase tracking-wider border", c.className)}>
      {c.label}
    </span>
  );
}
