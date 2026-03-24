import { useState } from "react";
import { Copy, Download, Check, Loader2 } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { fetchImages, registries } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Light as SyntaxHighlighter } from "react-syntax-highlighter";
import json from "react-syntax-highlighter/dist/esm/languages/hljs/json";
import { atomOneDark } from "react-syntax-highlighter/dist/esm/styles/hljs";
import { cn } from "@/lib/utils";

SyntaxHighlighter.registerLanguage("json", json);

const formats = [
  { id: "ocir", label: "Oracle OCIR JSON" },
  { id: "ecr", label: "ECR JSON" },
  { id: "csv", label: "CSV" },
];

export default function Export() {
  const [activeFormat, setActiveFormat] = useState("ocir");
  const [copied, setCopied] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["images", "all"],
    queryFn: () => fetchImages(),
  });

  const images = data?.images ?? [];

  const exportJSON = {
    version: "rgk/v1",
    registry: activeFormat === "ecr" ? "ecr-use1" : "ocir-fra",
    exported_at: new Date().toISOString(),
    images: images.map((img) => ({
      repo: img.repo,
      tag: img.tag,
      digest: img.digest,
      pinned: img.pinned,
      group: img.group,
      expires_in_days: img.expiresIn,
      last_keepalive: img.lastKeepalive,
    })),
  };

  const csvHeader = "repo,tag,digest,pinned,group,expires_in_days,last_keepalive";
  const csvRows = images.map((img) =>
    `${img.repo},${img.tag},${img.digest},${img.pinned},${img.group},${img.expiresIn},${img.lastKeepalive}`
  );
  const csvStr = [csvHeader, ...csvRows].join("\n");

  const jsonStr = JSON.stringify(exportJSON, null, 2);
  const content = activeFormat === "csv" ? csvStr : jsonStr;
  const lang = activeFormat === "csv" ? "plaintext" : "json";

  const handleCopy = () => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleDownload = () => {
    const ext = activeFormat === "csv" ? "csv" : "json";
    const blob = new Blob([content], { type: activeFormat === "csv" ? "text/csv" : "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `regikeep-export.${ext}`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold font-mono-data tracking-tight text-foreground">Export</h1>

      <div className="flex items-center gap-1 border-b border-border">
        {formats.map((f) => (
          <button
            key={f.id}
            onClick={() => setActiveFormat(f.id)}
            className={cn(
              "px-3 py-2 text-xs font-mono-data border-b-2 transition-colors",
              activeFormat === f.id ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-5 w-5 animate-spin text-primary mr-2" />
          <span className="font-mono-data text-xs text-muted-foreground">Loading data…</span>
        </div>
      ) : (
        <div className="border border-border rounded-md overflow-hidden">
          <div className="flex items-center justify-between px-3 py-1.5 bg-muted/50 border-b border-border">
            <span className="text-[10px] font-mono-data text-muted-foreground uppercase tracking-wider">Preview ({images.length} images)</span>
            <div className="flex gap-1">
              <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] font-mono-data" onClick={handleCopy}>
                {copied ? <Check className="h-3 w-3 mr-1" /> : <Copy className="h-3 w-3 mr-1" />}
                {copied ? "Copied" : "Copy"}
              </Button>
              <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] font-mono-data" onClick={handleDownload}>
                <Download className="h-3 w-3 mr-1" /> Download
              </Button>
            </div>
          </div>
          <SyntaxHighlighter
            language={lang}
            style={atomOneDark}
            customStyle={{ margin: 0, padding: "1rem", background: "hsl(216, 28%, 4%)", fontSize: "11px", lineHeight: "1.5" }}
          >
            {content}
          </SyntaxHighlighter>
        </div>
      )}
    </div>
  );
}
