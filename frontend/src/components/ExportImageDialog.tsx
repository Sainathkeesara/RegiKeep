import { useState } from "react";
import { Copy, Download, Archive, Loader2 } from "lucide-react";
import { RegistryImage, fetchRegistries, exportImage, archiveImage } from "@/lib/api";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "@/hooks/use-toast";

interface ExportImageDialogProps {
  image: RegistryImage;
  children: React.ReactNode;
}

export function ExportImageDialog({ image, children }: ExportImageDialogProps) {
  const [targetRegistry, setTargetRegistry] = useState("");
  const [exporting, setExporting] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const queryClient = useQueryClient();

  const { data: regData } = useQuery({
    queryKey: ["registries"],
    queryFn: fetchRegistries,
  });
  const registries = regData?.registries ?? [];
  const availableRegistries = registries.filter((r) => r.name !== image.registry);

  const handleExport = async () => {
    if (!targetRegistry) return;
    setExporting(true);
    try {
      await exportImage({
        imageId: image.id,
        sourceRegistry: image.registry,
        targetRegistry,
        repo: image.repo,
        tag: image.tag,
      });
      const target = registries.find((r) => r.name === targetRegistry);
      toast({ title: "Export initiated", description: `${image.repo}:${image.tag} → ${target?.name ?? targetRegistry}` });
    } catch (e) {
      toast({ title: "Export failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setExporting(false);
    }
  };

  const handleArchive = async () => {
    setArchiving(true);
    try {
      await archiveImage(image.id);
      queryClient.invalidateQueries({ queryKey: ["archives"] });
      toast({ title: "Archive complete", description: `${image.repo}:${image.tag} archived to S3.` });
    } catch (e) {
      toast({ title: "Archive failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setArchiving(false);
    }
  };

  const handleBackup = () => {
    const blob = new Blob(
      [JSON.stringify({ repo: image.repo, tag: image.tag, digest: image.digest, registry: image.registry, size: image.size, group: image.group, exportedAt: new Date().toISOString() }, null, 2)],
      { type: "application/json" }
    );
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${image.repo.replace("/", "-")}-${image.tag}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <Dialog>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-mono-data text-sm">Export / Archive Image</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="p-3 rounded bg-muted border border-border">
            <p className="font-mono-data text-xs text-foreground">{image.repo}:<span className="text-primary">{image.tag}</span></p>
            <p className="font-mono-data text-[10px] text-muted-foreground mt-1">{image.digest || "no digest"}</p>
          </div>

          <div className="space-y-2">
            <label className="font-mono-data text-xs text-muted-foreground uppercase tracking-wider">Export to Registry</label>
            <Select value={targetRegistry} onValueChange={setTargetRegistry}>
              <SelectTrigger className="h-8 text-xs font-mono-data">
                <SelectValue placeholder="Select target registry" />
              </SelectTrigger>
              <SelectContent>
                {availableRegistries.map((r) => (
                  <SelectItem key={r.id} value={r.name} className="text-xs font-mono-data">{r.name} ({r.registryType})</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button size="sm" className="w-full h-8 text-xs font-mono-data" disabled={!targetRegistry || exporting} onClick={handleExport}>
              <Copy className="h-3 w-3 mr-1" /> {exporting ? "Exporting..." : "Export to Registry"}
            </Button>
          </div>

          <div className="border-t border-border pt-3 space-y-2">
            <Button size="sm" className="w-full h-8 text-xs font-mono-data" onClick={handleArchive} disabled={archiving}>
              {archiving ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : <Archive className="h-3 w-3 mr-1" />}
              {archiving ? "Archiving..." : "Archive to S3"}
            </Button>
            <Button variant="outline" size="sm" className="w-full h-8 text-xs font-mono-data" onClick={handleBackup}>
              <Download className="h-3 w-3 mr-1" /> Download Backup (JSON)
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
