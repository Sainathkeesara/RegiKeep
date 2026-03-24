import { useState } from "react";
import { Pin, PinOff, Search, Filter, ArrowRightLeft } from "lucide-react";
import { RegistryImage, ImageStatus, pinImage, assignImageRegistry, fetchRegistries } from "@/lib/api";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { ExportImageDialog } from "@/components/ExportImageDialog";
import { TrivyScanButton } from "@/components/TrivyScanPanel";
import { useQueryClient, useQuery } from "@tanstack/react-query";
import { toast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";

interface ImageTableProps {
  images: RegistryImage[];
}

const filters: { label: string; value: ImageStatus | "all" }[] = [
  { label: "all", value: "all" },
  { label: "safe", value: "safe" },
  { label: "warning", value: "warning" },
  { label: "critical", value: "critical" },
  { label: "unpinned", value: "unpinned" },
];

export function ImageTable({ images }: ImageTableProps) {
  const [search, setSearch] = useState("");
  const [activeFilter, setActiveFilter] = useState<ImageStatus | "all">("all");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [pinningIds, setPinningIds] = useState<Set<string>>(new Set());
  const [assigningIds, setAssigningIds] = useState<Set<string>>(new Set());
  const queryClient = useQueryClient();

  const { data: regData } = useQuery({ queryKey: ["registries"], queryFn: fetchRegistries });
  const registryNames = (regData?.registries ?? []).map((r) => r.name);

  const handleAssignRegistry = async (id: string, registry: string) => {
    setAssigningIds((prev) => new Set(prev).add(id));
    try {
      await assignImageRegistry(id, registry);
      queryClient.invalidateQueries({ queryKey: ["images"] });
      toast({ title: `Image assigned to registry "${registry}"` });
    } catch (e) {
      toast({ title: "Failed to assign registry", description: (e as Error).message, variant: "destructive" });
    } finally {
      setAssigningIds((prev) => { const n = new Set(prev); n.delete(id); return n; });
    }
  };

  const filtered = images
    .filter((img) => activeFilter === "all" || img.status === activeFilter)
    .filter((img) =>
      `${img.repo}:${img.tag} ${img.digest} ${img.group}`.toLowerCase().includes(search.toLowerCase())
    );

  const handlePin = async (id: string, action: "pin" | "unpin") => {
    setPinningIds((prev) => new Set(prev).add(id));
    try {
      await pinImage(id, action);
      queryClient.invalidateQueries({ queryKey: ["images"] });
      toast({ title: `Image ${action}ned successfully` });
    } catch (e) {
      toast({ title: `Failed to ${action}`, description: (e as Error).message, variant: "destructive" });
    } finally {
      setPinningIds((prev) => { const n = new Set(prev); n.delete(id); return n; });
    }
  };

  const bulkPin = async () => {
    const ids = Array.from(selected);
    for (const id of ids) {
      await handlePin(id, "pin");
    }
    setSelected(new Set());
  };

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search images..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8 h-8 text-xs font-mono-data bg-muted border-border"
          />
        </div>
        <div className="flex items-center gap-1">
          <Filter className="h-3.5 w-3.5 text-muted-foreground mr-1" />
          {filters.map((f) => (
            <button
              key={f.value}
              onClick={() => setActiveFilter(f.value)}
              className={cn(
                "px-2 py-1 rounded text-[10px] font-mono-data uppercase tracking-wider transition-colors",
                activeFilter === f.value
                  ? "bg-primary/20 text-primary"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted"
              )}
            >
              {f.label}
            </button>
          ))}
        </div>
        {selected.size > 0 && (
          <Button size="sm" className="h-7 text-xs font-mono-data" onClick={bulkPin}>
            <Pin className="h-3 w-3 mr-1" /> Pin {selected.size} selected
          </Button>
        )}
      </div>

      <div className="border border-border rounded-md overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-muted/50 border-b border-border">
              <th className="w-8 p-2"><span className="sr-only">Select</span></th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Repo:Tag</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Registry</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Digest</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Region</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Size</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Status</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Last Pull</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Group</th>
              <th className="text-right p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Action</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((img) => (
              <tr key={img.id} className="border-b border-border hover:bg-muted/30 transition-colors">
                <td className="p-2 text-center">
                  <Checkbox
                    checked={selected.has(img.id)}
                    onCheckedChange={() => toggleSelect(img.id)}
                    className="border-border"
                  />
                </td>
                <td className="p-2 font-mono-data text-foreground">
                  {img.repo}:<span className="text-primary">{img.tag}</span>
                </td>
                <td className="p-2">
                  {img.registry ? (
                    <span className="px-1.5 py-0.5 bg-primary/10 rounded text-[10px] font-mono-data text-primary">{img.registry}</span>
                  ) : registryNames.length > 0 ? (
                    <select
                      disabled={assigningIds.has(img.id)}
                      defaultValue=""
                      onChange={(e) => { if (e.target.value) handleAssignRegistry(img.id, e.target.value); }}
                      className="h-6 text-[10px] font-mono-data bg-muted border border-destructive/50 rounded px-1 text-destructive cursor-pointer"
                      title="Image has no registry — assign one"
                    >
                      <option value="" disabled>assign…</option>
                      {registryNames.map((n) => <option key={n} value={n}>{n}</option>)}
                    </select>
                  ) : (
                    <span className="text-[10px] font-mono-data text-muted-foreground">—</span>
                  )}
                </td>
                <td className="p-2 font-mono-data text-muted-foreground">{img.digest ? img.digest.slice(0, 19) + "…" : "—"}</td>
                <td className="p-2 font-mono-data text-muted-foreground">{img.region}</td>
                <td className="p-2 font-mono-data text-muted-foreground">{img.size}</td>
                <td className="p-2">
                  <StatusBadge status={img.status} expiresIn={img.expiresIn} />
                  {img.lastError && (
                    <div className="mt-0.5 text-[9px] font-mono-data text-destructive truncate max-w-[200px]" title={img.lastError}>
                      {img.lastError}
                    </div>
                  )}
                </td>
                <td className="p-2 font-mono-data text-muted-foreground">
                  {img.lastKeepalive
                    ? new Date(img.lastKeepalive).toLocaleString()
                    : <span className="text-[10px] italic">never</span>}
                </td>
                <td className="p-2">
                  <span className="px-1.5 py-0.5 bg-muted rounded text-[10px] font-mono-data text-muted-foreground">{img.group || "—"}</span>
                </td>
                <td className="p-2 text-right flex items-center justify-end gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-[10px] font-mono-data"
                    disabled={pinningIds.has(img.id)}
                    onClick={() => handlePin(img.id, img.pinned ? "unpin" : "pin")}
                  >
                    {img.pinned ? <><PinOff className="h-3 w-3 mr-1" />UNPIN</> : <><Pin className="h-3 w-3 mr-1" />PIN</>}
                  </Button>
                  <ExportImageDialog image={img}>
                    <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] font-mono-data">
                      <ArrowRightLeft className="h-3 w-3 mr-1" /> EXPORT
                    </Button>
                  </ExportImageDialog>
                  <TrivyScanButton image={img} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && (
          <div className="p-8 text-center text-muted-foreground font-mono-data text-xs">
            No images match current filters.
          </div>
        )}
      </div>
    </div>
  );
}
