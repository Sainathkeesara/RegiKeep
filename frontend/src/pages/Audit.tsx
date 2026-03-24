import { useState } from "react";
import { ShieldCheck, ShieldAlert, Pin, Play } from "lucide-react";
import { runAudit, pinImage, AuditResult } from "@/lib/api";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { toast } from "@/hooks/use-toast";
import { useQueryClient } from "@tanstack/react-query";
import type { ImageStatus } from "@/lib/api";

export default function Audit() {
  const [hasRun, setHasRun] = useState(false);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<AuditResult[]>([]);
  const [summary, setSummary] = useState<{ totalScanned: number; atRisk: number; critical: number; warning: number; unpinned: number } | null>(null);
  const queryClient = useQueryClient();

  const handleRunAudit = async () => {
    setRunning(true);
    try {
      const data = await runAudit({ dryRun: true });
      setResults(data.results);
      setSummary(data.summary);
      setHasRun(true);
    } catch (e) {
      toast({ title: "Audit failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setRunning(false);
    }
  };

  const handlePinNow = async (imageId: string) => {
    try {
      await pinImage(imageId, "pin");
      setResults((prev) => prev.filter((r) => r.imageId !== imageId));
      queryClient.invalidateQueries({ queryKey: ["images"] });
      toast({ title: "Image pinned successfully" });
    } catch (e) {
      toast({ title: "Pin failed", description: (e as Error).message, variant: "destructive" });
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold font-mono-data tracking-tight text-foreground">Audit</h1>
        <Button size="sm" className="h-7 text-xs font-mono-data" onClick={handleRunAudit} disabled={running}>
          <Play className="h-3 w-3 mr-1" /> {running ? "Scanning…" : "Run Audit"}
        </Button>
      </div>

      {!hasRun && !running && (
        <div className="border border-border rounded-md bg-card p-12 text-center">
          <ShieldCheck className="h-10 w-10 text-muted-foreground mx-auto mb-3" />
          <p className="text-sm text-muted-foreground font-mono-data">Run a dry-run audit to see which images would be deleted by retention policies.</p>
        </div>
      )}

      {running && (
        <div className="border border-border rounded-md bg-card p-12 text-center">
          <div className="h-8 w-8 border-2 border-primary border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm text-muted-foreground font-mono-data">Scanning registries…</p>
        </div>
      )}

      {hasRun && !running && results.length === 0 && (
        <div className="border border-status-safe/30 rounded-md bg-status-safe/5 p-12 text-center">
          <ShieldCheck className="h-12 w-12 text-status-safe mx-auto mb-3" />
          <p className="text-lg font-mono-data font-semibold text-status-safe">All Clear</p>
          <p className="text-sm text-muted-foreground font-mono-data mt-1">No images at risk of deletion.</p>
        </div>
      )}

      {hasRun && !running && results.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-status-critical">
            <ShieldAlert className="h-4 w-4" />
            <span className="text-sm font-mono-data font-semibold">
              {summary?.atRisk ?? results.length} images at risk
              {summary && ` (${summary.critical} critical, ${summary.warning} warning, ${summary.unpinned} unpinned)`}
            </span>
          </div>
          <div className="border border-border rounded-md overflow-hidden">
            <table className="w-full text-xs">
              <thead>
                <tr className="bg-muted/50 border-b border-border">
                  <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Image</th>
                  <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Region</th>
                  <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Risk</th>
                  <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Expires In</th>
                  <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Recommendation</th>
                  <th className="text-right p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Action</th>
                </tr>
              </thead>
              <tbody>
                {results.map((r) => (
                  <tr key={r.imageId} className="border-b border-border bg-destructive/5 hover:bg-destructive/10 transition-colors">
                    <td className="p-2 font-mono-data text-foreground">{r.repo}:<span className="text-primary">{r.tag}</span></td>
                    <td className="p-2 font-mono-data text-muted-foreground">{r.region}</td>
                    <td className="p-2"><StatusBadge status={r.risk as ImageStatus} expiresIn={r.expiresIn} /></td>
                    <td className="p-2 font-mono-data text-muted-foreground">{r.expiresIn > 0 ? `${r.expiresIn}d` : "—"}</td>
                    <td className="p-2 font-mono-data text-muted-foreground max-w-xs truncate">{r.recommendation}</td>
                    <td className="p-2 text-right">
                      <Button
                        size="sm"
                        className="h-6 px-2 text-[10px] font-mono-data bg-status-safe/20 text-status-safe hover:bg-status-safe/30 border-0"
                        onClick={() => handlePinNow(r.imageId)}
                      >
                        <Pin className="h-3 w-3 mr-1" /> PIN NOW
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
