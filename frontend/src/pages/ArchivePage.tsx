import { useState } from "react";
import { Archive, RotateCcw, HardDrive, TrendingDown, Loader2, Trash2 } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchArchives, fetchGroups, restoreImage, deleteArchive, archiveImage, archiveGroup } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";

export default function ArchivePage() {
  const queryClient = useQueryClient();
  const [selectedGroup, setSelectedGroup] = useState("");
  const [archiving, setArchiving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [restoringId, setRestoringId] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ["archives"],
    queryFn: fetchArchives,
  });

  const { data: groupData } = useQuery({
    queryKey: ["groups"],
    queryFn: fetchGroups,
  });

  const archives = data?.archives ?? [];
  const groups = groupData?.groups ?? [];

  const handleArchiveGroup = async () => {
    if (!selectedGroup) {
      toast({ title: "Select a group", description: "Choose which group to archive.", variant: "destructive" });
      return;
    }
    setArchiving(true);
    try {
      await archiveGroup(selectedGroup);
      queryClient.invalidateQueries({ queryKey: ["archives"] });
      toast({ title: "Archive complete", description: `Group '${selectedGroup}' archived to S3.` });
    } catch (e) {
      toast({ title: "Archive failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setArchiving(false);
    }
  };

  const handleRestore = async (id: string) => {
    setRestoringId(id);
    try {
      await restoreImage(id);
      queryClient.invalidateQueries({ queryKey: ["archives"] });
      toast({ title: "Restore initiated" });
    } catch (e) {
      toast({ title: "Restore failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setRestoringId(null);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      await deleteArchive(id);
      queryClient.invalidateQueries({ queryKey: ["archives"] });
      toast({ title: "Archive deleted" });
    } catch (e) {
      toast({ title: "Delete failed", description: (e as Error).message, variant: "destructive" });
    } finally {
      setDeletingId(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-5 w-5 animate-spin text-primary mr-2" />
        <span className="font-mono-data text-xs text-muted-foreground">Loading archives...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8 text-center text-destructive font-mono-data text-xs">
        Failed to load archives: {(error as Error).message}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold font-mono-data tracking-tight text-foreground">Archive</h1>
        <div className="flex items-center gap-2">
          <Select value={selectedGroup} onValueChange={setSelectedGroup}>
            <SelectTrigger className="w-40 h-7 text-xs font-mono-data">
              <SelectValue placeholder="Select group" />
            </SelectTrigger>
            <SelectContent>
              {groups.map((g) => (
                <SelectItem key={g.id} value={g.name} className="text-xs font-mono-data">{g.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button size="sm" className="h-7 text-xs font-mono-data" onClick={handleArchiveGroup} disabled={archiving}>
            {archiving ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : <Archive className="h-3 w-3 mr-1" />}
            Archive Group
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="bg-card border border-border rounded-md p-4">
          <div className="flex items-center gap-2 mb-1">
            <HardDrive className="h-4 w-4 text-secondary" />
            <span className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Archived Images</span>
          </div>
          <p className="text-xl font-mono-data font-bold text-foreground">{data?.total ?? 0}</p>
        </div>
        <div className="bg-card border border-border rounded-md p-4">
          <div className="flex items-center gap-2 mb-1">
            <TrendingDown className="h-4 w-4 text-status-safe" />
            <span className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Original Size</span>
          </div>
          <p className="text-xl font-mono-data font-bold text-foreground">{data?.totalOriginalSize ?? "—"}</p>
        </div>
        <div className="bg-card border border-border rounded-md p-4">
          <div className="flex items-center gap-2 mb-1">
            <Archive className="h-4 w-4 text-primary" />
            <span className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Total Compressed</span>
          </div>
          <p className="text-xl font-mono-data font-bold text-foreground">{data?.totalCompressedSize ?? "—"}</p>
        </div>
      </div>

      <div className="border border-border rounded-md overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-muted/50 border-b border-border">
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Image</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Original</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Compressed</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Archived</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Backend</th>
              <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Status</th>
              <th className="text-right p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Actions</th>
            </tr>
          </thead>
          <tbody>
            {archives.map((img) => (
              <tr key={img.id} className="border-b border-border hover:bg-muted/30 transition-colors">
                <td className="p-2 font-mono-data text-foreground">{img.repo}:<span className="text-primary">{img.tag}</span></td>
                <td className="p-2 font-mono-data text-muted-foreground">{img.originalSize}</td>
                <td className="p-2 font-mono-data text-secondary">{img.compressedSize}</td>
                <td className="p-2 font-mono-data text-muted-foreground">{new Date(img.archivedAt).toLocaleDateString()}</td>
                <td className="p-2 font-mono-data text-muted-foreground uppercase">{img.storageBackend || "s3"}</td>
                <td className="p-2">
                  <span className={cn(
                    "px-2 py-0.5 rounded text-[10px] font-mono-data font-semibold uppercase tracking-wider border",
                    img.restoreStatus === "restored"
                      ? "bg-status-safe/15 text-status-safe border-status-safe/30"
                      : img.restorable
                      ? "bg-primary/15 text-primary border-primary/30"
                      : "bg-muted text-muted-foreground border-border"
                  )}>
                    {img.restoreStatus === "restored" ? "RESTORED" : img.restorable ? "RESTORABLE" : "UNAVAILABLE"}
                  </span>
                </td>
                <td className="p-2 text-right space-x-1">
                  {img.restorable && img.restoreStatus !== "restored" && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 px-2 text-[10px] font-mono-data"
                      onClick={() => handleRestore(img.id)}
                      disabled={restoringId === img.id}
                    >
                      {restoringId === img.id ? <Loader2 className="h-3 w-3 animate-spin" /> : <RotateCcw className="h-3 w-3 mr-1" />}
                      Restore
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-[10px] font-mono-data text-destructive hover:text-destructive"
                    onClick={() => handleDelete(img.id)}
                    disabled={deletingId === img.id}
                  >
                    {deletingId === img.id ? <Loader2 className="h-3 w-3 animate-spin" /> : <Trash2 className="h-3 w-3 mr-1" />}
                    Delete
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {archives.length === 0 && (
          <div className="p-8 text-center text-muted-foreground font-mono-data text-xs">
            No archived images. Select a group and click "Archive Group" to get started.
          </div>
        )}
      </div>
    </div>
  );
}
