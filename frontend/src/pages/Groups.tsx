import { useState } from "react";
import { Plus, ChevronRight, Loader2, Trash2 } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  fetchImages, fetchGroups, createGroup, deleteGroup, enableGroup, disableGroup,
  setImageGroup, removeImageGroup, deleteImage, RegistryImage, Group,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { StatusBadge } from "@/components/StatusBadge";
import { toast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";

export default function Groups() {
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null);
  const [showUnassigned, setShowUnassigned] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newInterval, setNewInterval] = useState("7d");
  const [newStrategy, setNewStrategy] = useState("pull");
  const queryClient = useQueryClient();

  const { data: groupsData, isLoading: groupsLoading } = useQuery({
    queryKey: ["groups"],
    queryFn: fetchGroups,
  });

  const { data: imagesData, isLoading: imagesLoading } = useQuery({
    queryKey: ["images", "all"],
    queryFn: () => fetchImages(),
  });

  const isLoading = groupsLoading || imagesLoading;
  const backendGroups: Group[] = groupsData?.groups ?? [];
  const images = imagesData?.images ?? [];

  const imagesByGroup = new Map<string, RegistryImage[]>();
  const unassigned: RegistryImage[] = [];
  for (const img of images) {
    if (!img.group || img.group === "") {
      unassigned.push(img);
    } else {
      const existing = imagesByGroup.get(img.group) ?? [];
      existing.push(img);
      imagesByGroup.set(img.group, existing);
    }
  }

  const allGroupNames = backendGroups.map((g) => g.name);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["groups"] });
    queryClient.invalidateQueries({ queryKey: ["images"] });
  };

  const removeMut = useMutation({
    mutationFn: (id: string) => removeImageGroup(id),
    onSuccess: () => { invalidate(); toast({ title: "Image removed from group" }); },
    onError: (e) => toast({ title: "Failed", description: (e as Error).message, variant: "destructive" }),
  });
  const addMut = useMutation({
    mutationFn: ({ imageId, groupName }: { imageId: string; groupName: string }) => setImageGroup(imageId, groupName),
    onSuccess: (_d, v) => { invalidate(); toast({ title: `Image added to ${v.groupName}` }); },
    onError: (e) => toast({ title: "Failed", description: (e as Error).message, variant: "destructive" }),
  });
  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteImage(id),
    onSuccess: () => { invalidate(); toast({ title: "Image deleted" }); },
    onError: (e) => toast({ title: "Failed", description: (e as Error).message, variant: "destructive" }),
  });
  const createMut = useMutation({
    mutationFn: () => createGroup(newName, newInterval, newStrategy),
    onSuccess: () => { invalidate(); toast({ title: `Group '${newName}' created` }); setNewName(""); setShowCreate(false); },
    onError: (e) => toast({ title: "Create failed", description: (e as Error).message, variant: "destructive" }),
  });
  const deleteGroupMut = useMutation({
    mutationFn: (id: string) => deleteGroup(id),
    onSuccess: () => { invalidate(); toast({ title: "Group deleted" }); },
    onError: (e) => toast({ title: "Failed", description: (e as Error).message, variant: "destructive" }),
  });
  const toggleGroupMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => enabled ? enableGroup(id) : disableGroup(id),
    onSuccess: (_d, v) => { invalidate(); toast({ title: `Group ${v.enabled ? "enabled" : "disabled"}` }); },
    onError: (e) => toast({ title: "Failed", description: (e as Error).message, variant: "destructive" }),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-5 w-5 animate-spin text-primary mr-2" />
        <span className="font-mono-data text-xs text-muted-foreground">Loading groups...</span>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold font-mono-data tracking-tight text-foreground">Groups</h1>
        <div className="flex items-center gap-2">
          {unassigned.length > 0 && (
            <Button variant="outline" size="sm" className="h-7 text-xs font-mono-data" onClick={() => setShowUnassigned(!showUnassigned)}>
              {unassigned.length} unassigned
            </Button>
          )}
          <Button size="sm" className="h-7 text-xs font-mono-data" onClick={() => setShowCreate(!showCreate)}>
            <Plus className="h-3 w-3 mr-1" /> New Group
          </Button>
        </div>
      </div>

      {showCreate && (
        <div className="border border-border rounded-md bg-card p-4 space-y-3">
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Name</label>
              <Input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="production" className="h-8 text-xs font-mono-data mt-1" />
            </div>
            <div>
              <label className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Interval</label>
              <Input value={newInterval} onChange={(e) => setNewInterval(e.target.value)} placeholder="7d" className="h-8 text-xs font-mono-data mt-1" />
            </div>
            <div>
              <label className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Strategy</label>
              <select value={newStrategy} onChange={(e) => setNewStrategy(e.target.value)}
                className="mt-1 w-full h-8 text-xs font-mono-data bg-muted border border-border rounded-md px-2">
                <option value="pull">pull — reset "last pulled" timer</option>
                <option value="retag">retag — re-tag to reset age</option>
                <option value="native">native — registry lock/pin</option>
              </select>
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" className="h-7 text-xs font-mono-data" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button size="sm" className="h-7 text-xs font-mono-data" onClick={() => createMut.mutate()} disabled={!newName.trim() || createMut.isPending}>
              {createMut.isPending ? <Loader2 className="h-3 w-3 animate-spin mr-1" /> : null}Create
            </Button>
          </div>
        </div>
      )}

      {showUnassigned && unassigned.length > 0 && (
        <div className="border border-dashed border-border rounded-md bg-card overflow-hidden">
          <div className="px-4 py-2 border-b border-border bg-muted/30">
            <span className="font-mono-data text-xs font-semibold text-muted-foreground">Unassigned Images ({unassigned.length})</span>
          </div>
          <div className="p-3">
            <table className="w-full text-xs"><tbody>
              {unassigned.map((img) => (
                <tr key={img.id} className="border-b border-border/30 last:border-0">
                  <td className="p-1.5 font-mono-data text-foreground">{img.repo}:<span className="text-primary">{img.tag}</span></td>
                  <td className="p-1.5"><StatusBadge status={img.status} expiresIn={img.expiresIn} /></td>
                  <td className="p-1.5 text-right flex items-center justify-end gap-1">
                    {allGroupNames.map((gn) => (
                      <Button key={gn} variant="ghost" size="sm" className="h-5 px-1.5 text-[10px] font-mono-data" disabled={addMut.isPending}
                        onClick={() => addMut.mutate({ imageId: img.id, groupName: gn })}>
                        <ChevronRight className="h-2.5 w-2.5 mr-0.5" />{gn}
                      </Button>
                    ))}
                    <Button variant="ghost" size="sm" className="h-5 w-5 p-0 text-muted-foreground hover:text-destructive"
                      title="Delete from RegiKeep" disabled={deleteMut.isPending} onClick={() => deleteMut.mutate(img.id)}>
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody></table>
          </div>
        </div>
      )}

      {backendGroups.length === 0 && unassigned.length === 0 && (
        <div className="p-8 text-center text-muted-foreground font-mono-data text-xs">
          No groups yet. Click "New Group" to create one.
        </div>
      )}

      <div className="space-y-2">
        {backendGroups.map((group) => {
          const groupImages = imagesByGroup.get(group.name) ?? [];
          const isExpanded = expandedGroup === group.name;
          const healthyCount = groupImages.filter((i) => i.status === "safe" || i.status === "success").length;
          const healthPercent = groupImages.length > 0 ? Math.round((healthyCount / groupImages.length) * 100) : 0;

          return (
            <div key={group.id} className={cn("border rounded-md bg-card overflow-hidden", group.enabled ? "border-border" : "border-border/50 opacity-60")}>
              <div className="flex items-center gap-3 p-3 cursor-pointer hover:bg-muted/30 transition-colors"
                onClick={() => setExpandedGroup(isExpanded ? null : group.name)}>
                <ChevronRight className={cn("h-4 w-4 text-muted-foreground transition-transform", isExpanded && "rotate-90")} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono-data text-sm font-semibold text-foreground">{group.name}</span>
                    {!group.enabled && <span className="text-[9px] font-mono-data px-1.5 py-0.5 bg-muted rounded text-muted-foreground">DISABLED</span>}
                  </div>
                  <div className="flex items-center gap-3 mt-0.5 text-[10px] font-mono-data text-muted-foreground">
                    <span>images: {groupImages.length}</span>
                    <span>interval: {group.interval}</span>
                    <span>strategy: {group.strategy}</span>
                    <span>health: {healthPercent}%</span>
                  </div>
                </div>
                <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                  <div className="h-1.5 w-16 bg-muted rounded-full overflow-hidden">
                    <div className="h-full bg-status-safe rounded-full" style={{ width: `${healthPercent}%` }} />
                  </div>
                  <Switch checked={group.enabled} onCheckedChange={(v) => toggleGroupMut.mutate({ id: group.id, enabled: v })} className="scale-75" />
                  <Button variant="ghost" size="sm" className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                    title="Delete group" disabled={deleteGroupMut.isPending} onClick={() => deleteGroupMut.mutate(group.id)}>
                    <Trash2 className="h-3 w-3" />
                  </Button>
                </div>
              </div>
              {isExpanded && (
                <div className="border-t border-border p-3 bg-muted/20">
                  {groupImages.length === 0 ? (
                    <div className="text-center text-muted-foreground font-mono-data text-xs py-4">
                      No images in this group.
                    </div>
                  ) : (
                    <table className="w-full text-xs"><thead><tr className="text-left">
                      <th className="p-1.5 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Image</th>
                      <th className="p-1.5 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Status</th>
                      <th className="p-1.5 font-mono-data font-medium text-muted-foreground uppercase tracking-wider text-[10px]">Last Keepalive</th>
                      <th className="p-1.5 w-20"></th>
                    </tr></thead><tbody>
                      {groupImages.map((img) => (
                        <tr key={img.id} className="border-t border-border/50">
                          <td className="p-1.5 font-mono-data text-foreground">{img.repo}:<span className="text-primary">{img.tag}</span></td>
                          <td className="p-1.5"><StatusBadge status={img.status} expiresIn={img.expiresIn} /></td>
                          <td className="p-1.5 font-mono-data text-muted-foreground">{img.lastKeepalive ? new Date(img.lastKeepalive).toLocaleString() : "never"}</td>
                          <td className="p-1.5 flex items-center gap-0.5">
                            <Button variant="ghost" size="sm" className="h-5 px-1 text-[9px] font-mono-data text-muted-foreground hover:text-foreground"
                              title="Remove from group" disabled={removeMut.isPending}
                              onClick={(e) => { e.stopPropagation(); removeMut.mutate(img.id); }}>unassign</Button>
                            <Button variant="ghost" size="sm" className="h-5 w-5 p-0 text-muted-foreground hover:text-destructive"
                              title="Delete from RegiKeep" disabled={deleteMut.isPending}
                              onClick={(e) => { e.stopPropagation(); deleteMut.mutate(img.id); }}>
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody></table>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
