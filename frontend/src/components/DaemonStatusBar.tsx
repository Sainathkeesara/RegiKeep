import { Activity, Clock, Play, Square } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { fetchDaemonStatus, controlDaemon } from "@/lib/api";
import { Button } from "@/components/ui/button";

export function DaemonStatusBar() {
  const queryClient = useQueryClient();

  const { data } = useQuery({
    queryKey: ["daemon-status"],
    queryFn: fetchDaemonStatus,
    refetchInterval: 15000,
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: (action: "start" | "stop") => controlDaemon(action),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["daemon-status"] });
    },
  });

  const running = data?.running ?? false;
  const lastRun = data?.lastRun ?? null;
  const nextRun = data?.nextRun ?? null;

  const formatTime = (iso: string) => {
    const d = new Date(iso);
    return d.toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false });
  };

  return (
    <div className="flex items-center gap-4 px-4 py-2 bg-card border border-border rounded-md font-mono-data text-xs">
      <div className="flex items-center gap-1.5">
        <div className={`h-2 w-2 rounded-full ${running ? "bg-status-safe" : "bg-muted-foreground"}`} />
        <span className="text-muted-foreground">daemon:</span>
        <span className={running ? "text-status-safe" : "text-muted-foreground"}>
          {data === undefined ? "awaiting backend" : running ? "running" : "stopped"}
        </span>
      </div>
      <div className="h-3 w-px bg-border" />
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <Clock className="h-3 w-3" />
        <span>last: {lastRun ? formatTime(lastRun) : "—"}</span>
      </div>
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <Activity className="h-3 w-3" />
        <span>next: {nextRun ? formatTime(nextRun) : "—"}</span>
      </div>
      <div className="ml-auto">
        <Button
          variant="outline"
          size="sm"
          className="h-6 px-2 text-xs font-mono-data border-border"
          onClick={() => mutation.mutate(running ? "stop" : "start")}
          disabled={mutation.isPending}
        >
          {running ? <Square className="h-3 w-3 mr-1" /> : <Play className="h-3 w-3 mr-1" />}
          {running ? "stop" : "start"}
        </Button>
      </div>
    </div>
  );
}
