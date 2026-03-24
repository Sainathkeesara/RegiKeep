import { useState } from "react";
import { Container, Pin, AlertTriangle, AlertOctagon, Loader2 } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { fetchImages, fetchRegistries } from "@/lib/api";
import { StatCard } from "@/components/StatCard";
import { DaemonStatusBar } from "@/components/DaemonStatusBar";
import { ImageTable } from "@/components/ImageTable";
import { DockerHubSearch } from "@/components/DockerHubSearch";
import { cn } from "@/lib/utils";

export default function Dashboard() {
  const [activeRegistry, setActiveRegistry] = useState("all");

  const isDockerHubSearch = activeRegistry === "dockerhub-search";

  // Load registries from backend
  const { data: regData } = useQuery({
    queryKey: ["registries"],
    queryFn: fetchRegistries,
  });
  const registries = regData?.registries ?? [];

  const { data, isLoading, error } = useQuery({
    queryKey: ["images", activeRegistry],
    queryFn: () =>
      fetchImages(activeRegistry !== "all" ? { registry: activeRegistry } : undefined),
    enabled: !isDockerHubSearch,
  });

  const images = data?.images ?? [];
  const totalImages = images.length;
  const pinnedImages = images.filter((i) => i.pinned).length;
  const expiringImages = images.filter((i) => i.expiresIn <= 7 && i.expiresIn > 1).length;
  const criticalImages = images.filter((i) => i.expiresIn <= 1 && i.status !== "unpinned").length;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold font-mono-data tracking-tight text-foreground">Dashboard</h1>
        <span className="font-mono-data text-[10px] text-muted-foreground uppercase tracking-widest">rgk v1.0</span>
      </div>

      <DaemonStatusBar />

      {!isDockerHubSearch && (
        <div className="grid grid-cols-4 gap-3">
          <StatCard label="Total Images" value={isLoading ? "…" : totalImages} icon={Container} />
          <StatCard label="Protected (Pinned)" value={isLoading ? "…" : pinnedImages} icon={Pin} variant="safe" />
          <StatCard label="Expiring ≤7d" value={isLoading ? "…" : expiringImages} icon={AlertTriangle} variant="warning" />
          <StatCard label="Critical ≤1d" value={isLoading ? "…" : criticalImages} icon={AlertOctagon} variant="critical" />
        </div>
      )}

      <div className="flex items-center gap-1 border-b border-border">
        <button
          onClick={() => setActiveRegistry("all")}
          className={cn(
            "px-3 py-2 text-xs font-mono-data border-b-2 transition-colors",
            activeRegistry === "all"
              ? "border-primary text-primary"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
        >
          All Registries
        </button>
        {registries.map((r) => (
          <button
            key={r.id}
            onClick={() => setActiveRegistry(r.name)}
            className={cn(
              "px-3 py-2 text-xs font-mono-data border-b-2 transition-colors",
              activeRegistry === r.name
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {r.name} ({r.registryType})
          </button>
        ))}
        <button
          onClick={() => setActiveRegistry("dockerhub-search")}
          className={cn(
            "px-3 py-2 text-xs font-mono-data border-b-2 transition-colors ml-auto",
            isDockerHubSearch
              ? "border-primary text-primary"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
        >
          Search Docker Hub
        </button>
      </div>

      {isDockerHubSearch ? (
        <DockerHubSearch />
      ) : isLoading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-5 w-5 animate-spin text-primary mr-2" />
          <span className="font-mono-data text-xs text-muted-foreground">Loading images…</span>
        </div>
      ) : error ? (
        <div className="p-8 text-center text-destructive font-mono-data text-xs">
          Failed to load images: {(error as Error).message}
        </div>
      ) : (
        <ImageTable images={images} />
      )}
    </div>
  );
}
