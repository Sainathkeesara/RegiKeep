import { useState } from "react";
import { Shield, ShieldAlert, ShieldCheck, Loader2, AlertTriangle } from "lucide-react";
import { RegistryImage, runTrivyScan, ScanResult } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { toast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";

const severityColor: Record<string, string> = {
  CRITICAL: "text-status-critical",
  HIGH: "text-status-warning",
  MEDIUM: "text-primary",
  LOW: "text-muted-foreground",
};

const severityBg: Record<string, string> = {
  CRITICAL: "bg-status-critical/20",
  HIGH: "bg-status-warning/20",
  MEDIUM: "bg-primary/20",
  LOW: "bg-muted",
};

interface TrivyScanButtonProps {
  image: RegistryImage;
}

export function TrivyScanButton({ image }: TrivyScanButtonProps) {
  const [scanning, setScanning] = useState(false);
  const [result, setResult] = useState<ScanResult | null>(null);

  const handleScan = async () => {
    setScanning(true);
    try {
      const data = await runTrivyScan({ repo: image.repo, tag: image.tag });
      setResult(data);
    } catch (e) {
      toast({ title: "Scan failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setScanning(false);
    }
  };

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] font-mono-data" onClick={handleScan}>
          <Shield className="h-3 w-3 mr-1" /> SCAN
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="font-mono-data text-sm flex items-center gap-2">
            <Shield className="h-4 w-4" /> Trivy Scan — {image.repo}:{image.tag}
          </DialogTitle>
        </DialogHeader>

        {scanning && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-primary mr-2" />
            <span className="font-mono-data text-xs text-muted-foreground">Scanning vulnerabilities...</span>
          </div>
        )}

        {result && !scanning && (
          <div className="space-y-4">
            <div className="grid grid-cols-4 gap-2">
              {[
                { label: "Critical", count: result.totalCritical, color: "text-status-critical", bg: "bg-status-critical/20", icon: ShieldAlert },
                { label: "High", count: result.totalHigh, color: "text-status-warning", bg: "bg-status-warning/20", icon: AlertTriangle },
                { label: "Medium", count: result.totalMedium, color: "text-primary", bg: "bg-primary/20", icon: Shield },
                { label: "Low", count: result.totalLow, color: "text-muted-foreground", bg: "bg-muted", icon: ShieldCheck },
              ].map((s) => (
                <div key={s.label} className={cn("rounded p-2 text-center border border-border", s.bg)}>
                  <s.icon className={cn("h-4 w-4 mx-auto mb-1", s.color)} />
                  <div className={cn("font-mono-data text-lg font-bold", s.color)}>{s.count}</div>
                  <div className="font-mono-data text-[10px] text-muted-foreground uppercase">{s.label}</div>
                </div>
              ))}
            </div>

            <div className="border border-border rounded-md overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className="bg-muted/50 border-b border-border">
                    <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">CVE</th>
                    <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Severity</th>
                    <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Package</th>
                    <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Installed</th>
                    <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Fixed</th>
                  </tr>
                </thead>
                <tbody>
                  {result.vulnerabilities.map((v) => (
                    <tr key={v.id} className="border-b border-border hover:bg-muted/30">
                      <td className="p-2 font-mono-data text-foreground">{v.id}</td>
                      <td className="p-2">
                        <span className={cn("px-1.5 py-0.5 rounded text-[10px] font-mono-data uppercase", severityBg[v.severity], severityColor[v.severity])}>
                          {v.severity}
                        </span>
                      </td>
                      <td className="p-2 font-mono-data text-muted-foreground">{v.package}</td>
                      <td className="p-2 font-mono-data text-muted-foreground">{v.installedVersion}</td>
                      <td className="p-2 font-mono-data text-status-safe">{v.fixedVersion}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <p className="font-mono-data text-[10px] text-muted-foreground">
              Scanned at {new Date(result.scannedAt).toLocaleString()}
            </p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
