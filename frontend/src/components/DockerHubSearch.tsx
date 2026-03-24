import { useState } from "react";
import { Search, Download, Star, Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { fetchRegistries, fetchGroups, searchDockerHub as apiSearchDockerHub, pushToRegistry } from "@/lib/api";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "@/hooks/use-toast";

interface DockerHubResult {
  repo_name: string;
  short_description: string;
  star_count: number;
  pull_count: number;
  is_official: boolean;
}

export function DockerHubSearch() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<DockerHubResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [targetRegistry, setTargetRegistry] = useState("");
  const [targetGroup, setTargetGroup] = useState("");
  const [pushingRepo, setPushingRepo] = useState<string | null>(null);
  const [tagInputs, setTagInputs] = useState<Record<string, string>>({});
  const queryClient = useQueryClient();

  const { data: regData } = useQuery({
    queryKey: ["registries"],
    queryFn: fetchRegistries,
  });
  const registries = regData?.registries ?? [];

  const { data: groupData } = useQuery({
    queryKey: ["groups"],
    queryFn: fetchGroups,
  });
  const groups = groupData?.groups ?? [];

  const searchDockerHub = async () => {
    if (!query.trim()) return;
    setLoading(true);
    try {
      const res = await apiSearchDockerHub(query);
      setResults(res || []);
    } catch {
      toast({ title: "Search failed", description: "Could not reach Docker Hub API.", variant: "destructive" });
    } finally {
      setLoading(false);
    }
  };

  const getTag = (repoName: string) => tagInputs[repoName] || "latest";

  const handlePull = async (repoName: string) => {
    if (!targetRegistry) {
      toast({ title: "Select a target registry", description: "Choose where to push this image.", variant: "destructive" });
      return;
    }

    const tag = getTag(repoName);
    setPushingRepo(repoName);

    toast({
      title: "Push started",
      description: `Pulling ${repoName}:${tag} from Docker Hub → ${targetRegistry}...`,
    });

    try {
      const result = await pushToRegistry({
        image: `${repoName}:${tag}`,
        targetRegistry,
        group: targetGroup || undefined,
      });

      toast({
        title: "Push complete",
        description: `${repoName}:${tag} → ${targetRegistry}. ${result.blobsCopied} blobs copied, ${result.blobsSkipped} skipped.`,
      });

      queryClient.invalidateQueries({ queryKey: ["images"] });
    } catch (err: any) {
      toast({
        title: "Push failed",
        description: err.message || "Unknown error",
        variant: "destructive",
      });
    } finally {
      setPushingRepo(null);
    }
  };

  const formatPulls = (n: number) => (n >= 1_000_000 ? `${(n / 1_000_000).toFixed(1)}M` : n >= 1_000 ? `${(n / 1_000).toFixed(1)}K` : String(n));

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search Docker Hub..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && searchDockerHub()}
            className="pl-8 h-8 text-xs font-mono-data bg-muted border-border"
          />
        </div>
        <Button size="sm" className="h-8 text-xs font-mono-data" onClick={searchDockerHub} disabled={loading}>
          {loading ? <Loader2 className="h-3 w-3 animate-spin" /> : "Search"}
        </Button>
        <Select value={targetRegistry} onValueChange={setTargetRegistry}>
          <SelectTrigger className="w-48 h-8 text-xs font-mono-data">
            <SelectValue placeholder="Target registry" />
          </SelectTrigger>
          <SelectContent>
            {registries.map((r) => (
              <SelectItem key={r.id} value={r.name} className="text-xs font-mono-data">{r.name} ({r.registryType})</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={targetGroup} onValueChange={setTargetGroup}>
          <SelectTrigger className="w-40 h-8 text-xs font-mono-data">
            <SelectValue placeholder="Group (optional)" />
          </SelectTrigger>
          <SelectContent>
            {groups.map((g) => (
              <SelectItem key={g.id} value={g.name} className="text-xs font-mono-data">{g.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {results.length > 0 && (
        <div className="border border-border rounded-md overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="bg-muted/50 border-b border-border">
                <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Repository</th>
                <th className="text-left p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Description</th>
                <th className="text-right p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Stars</th>
                <th className="text-right p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Pulls</th>
                <th className="text-center p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Tag</th>
                <th className="text-right p-2 font-mono-data font-medium text-muted-foreground uppercase tracking-wider">Action</th>
              </tr>
            </thead>
            <tbody>
              {results.map((r) => (
                <tr key={r.repo_name} className="border-b border-border hover:bg-muted/30 transition-colors">
                  <td className="p-2 font-mono-data text-foreground">
                    {r.repo_name}
                    {r.is_official && <span className="ml-1.5 px-1 py-0.5 rounded text-[9px] bg-primary/20 text-primary uppercase">official</span>}
                  </td>
                  <td className="p-2 font-mono-data text-muted-foreground max-w-xs truncate">{r.short_description || "—"}</td>
                  <td className="p-2 font-mono-data text-muted-foreground text-right">
                    <Star className="inline h-3 w-3 mr-0.5" />{r.star_count}
                  </td>
                  <td className="p-2 font-mono-data text-muted-foreground text-right">{formatPulls(r.pull_count)}</td>
                  <td className="p-2 text-center">
                    <Input
                      placeholder="latest"
                      value={tagInputs[r.repo_name] || ""}
                      onChange={(e) => setTagInputs((prev) => ({ ...prev, [r.repo_name]: e.target.value }))}
                      className="h-6 w-24 text-[10px] font-mono-data bg-muted border-border inline-block"
                    />
                  </td>
                  <td className="p-2 text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 px-2 text-[10px] font-mono-data"
                      onClick={() => handlePull(r.repo_name)}
                      disabled={pushingRepo === r.repo_name}
                    >
                      {pushingRepo === r.repo_name ? (
                        <><Loader2 className="h-3 w-3 mr-1 animate-spin" /> PUSHING</>
                      ) : (
                        <><Download className="h-3 w-3 mr-1" /> PULL</>
                      )}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {results.length === 0 && !loading && query && (
        <div className="p-8 text-center text-muted-foreground font-mono-data text-xs">
          No results. Try a different search term.
        </div>
      )}
    </div>
  );
}
