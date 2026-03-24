// API service layer — all calls go to Supabase Edge Functions
// Set VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY in your environment

const BASE_URL = import.meta.env.VITE_SUPABASE_URL
  ? `${import.meta.env.VITE_SUPABASE_URL}/functions/v1`
  : "";
const ANON_KEY = import.meta.env.VITE_SUPABASE_ANON_KEY || "";

function headers(extra?: Record<string, string>): HeadersInit {
  return {
    "Content-Type": "application/json",
    ...(ANON_KEY ? { apikey: ANON_KEY, Authorization: `Bearer ${ANON_KEY}` } : {}),
    ...extra,
  };
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: headers(init?.headers as Record<string, string>),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `API error ${res.status}`);
  }
  return res.json();
}

// ─── Registry Images ───────────────────────────────────────

export interface RegistryImage {
  id: string;
  repo: string;
  tag: string;
  digest: string;
  region: string;
  size: string;
  group: string;
  pinned: boolean;
  expiresIn: number;
  lastKeepalive: string;
  status: "safe" | "warning" | "critical" | "unpinned" | "failure" | "success" | string;
  registry: string;
  lastError?: string;
}

export type ImageStatus = RegistryImage["status"];

export interface Registry {
  id: string;
  name: string;
  region: string;
}

export async function fetchImages(params?: {
  registry?: string;
  status?: string;
  search?: string;
}): Promise<{ images: RegistryImage[]; total: number }> {
  const q = new URLSearchParams();
  if (params?.registry) q.set("registry", params.registry);
  if (params?.status) q.set("status", params.status);
  if (params?.search) q.set("search", params.search);
  const qs = q.toString() ? `?${q.toString()}` : "";
  return request(`/registry-images${qs}`);
}

export async function pinImage(imageId: string, action: "pin" | "unpin") {
  return request<{ success: boolean; imageId: string; pinned: boolean }>("/registry-images", {
    method: "POST",
    body: JSON.stringify({ imageId, action }),
  });
}

export async function setImageGroup(imageId: string, groupName: string) {
  return request<{ success: boolean }>("/registry-images", {
    method: "POST",
    body: JSON.stringify({ imageId, action: "set-group", groupName }),
  });
}

export async function removeImageGroup(imageId: string) {
  return request<{ success: boolean }>("/registry-images", {
    method: "POST",
    body: JSON.stringify({ imageId, action: "remove-group" }),
  });
}

const API_BASE = import.meta.env.VITE_SUPABASE_URL || "";

export async function deleteImage(imageId: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/images/${imageId}`, {
    method: "DELETE",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Delete error ${res.status}`);
}

export async function assignImageRegistry(imageId: string, registry: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/images/${imageId}/registry`, {
    method: "PATCH",
    headers: headers(),
    body: JSON.stringify({ registry }),
  });
  if (!res.ok) throw new Error(`Assign registry error ${res.status}`);
}

// ─── Groups ───────────────────────────────────────────────

export interface Group {
  id: string;
  name: string;
  interval: string;
  strategy: string;
  enabled: boolean;
  createdAt: string;
}

export async function fetchGroups(): Promise<{ groups: Group[] }> {
  const res = await fetch(`${API_BASE}/api/v1/groups`, { headers: headers() });
  if (!res.ok) throw new Error(`Groups error ${res.status}`);
  return res.json();
}

export async function createGroup(name: string, interval: string, strategy: string): Promise<Group> {
  const res = await fetch(`${API_BASE}/api/v1/groups`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify({ name, interval, strategy }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Create group error ${res.status}`);
  }
  return res.json();
}

export async function deleteGroup(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/groups/${id}`, {
    method: "DELETE",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Delete group error ${res.status}`);
}

export async function enableGroup(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/groups/${id}/enable`, {
    method: "POST",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Enable group error ${res.status}`);
}

export async function disableGroup(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/groups/${id}/disable`, {
    method: "POST",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Disable group error ${res.status}`);
}

// ─── Docker Hub Search (proxied through backend) ──────────

export async function searchDockerHub(query: string): Promise<any[]> {
  const res = await fetch(`${API_BASE}/api/v1/dockerhub/search?q=${encodeURIComponent(query)}`, {
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Docker Hub search error ${res.status}`);
  const data = await res.json();
  return data.results ?? [];
}

// ─── Audit ─────────────────────────────────────────────────

export interface AuditResult {
  imageId: string;
  repo: string;
  tag: string;
  region: string;
  expiresIn: number;
  status: string;
  risk: "critical" | "warning" | "unpinned";
  recommendation: string;
}

export interface AuditResponse {
  dryRun: boolean;
  timestamp: string;
  summary: {
    totalScanned: number;
    atRisk: number;
    critical: number;
    warning: number;
    unpinned: number;
  };
  results: AuditResult[];
}

export async function runAudit(opts?: { dryRun?: boolean; registryFilter?: string }): Promise<AuditResponse> {
  return request("/audit", {
    method: "POST",
    body: JSON.stringify({ dryRun: opts?.dryRun ?? true, registryFilter: opts?.registryFilter }),
  });
}

// ─── Keepalive ─────────────────────────────────────────────

export interface KeepaliveResult {
  imageId: string;
  repo: string;
  tag: string;
  strategy: string;
  success: boolean;
  newExpiry: string;
  error?: string;
}

export interface KeepaliveResponse {
  timestamp: string;
  strategy: string;
  processed: number;
  results: KeepaliveResult[];
}

export async function triggerKeepalive(opts: {
  imageIds?: string[];
  group?: string;
  strategy?: "pull" | "retag" | "native";
}): Promise<KeepaliveResponse> {
  return request("/keepalive", {
    method: "POST",
    body: JSON.stringify(opts),
  });
}

// ─── Archive ───────────────────────────────────────────────

export interface ArchivedImage {
  id: string;
  repo: string;
  tag: string;
  compressedSize: string;
  originalSize: string;
  archivedAt: string;
  restorable: boolean;
  restoreStatus?: "idle" | "restoring" | "restored";
  storageBackend?: string;
}

export interface ArchiveListResponse {
  archives: ArchivedImage[];
  total: number;
  totalCompressedSize: string;
  totalOriginalSize: string;
}

export async function fetchArchives(): Promise<ArchiveListResponse> {
  return request("/archive");
}

export async function archiveImage(imageId: string) {
  return request<{ success: boolean }>("/archive", {
    method: "POST",
    body: JSON.stringify({ imageId, action: "archive" }),
  });
}

export async function restoreImage(imageId: string) {
  return request<{ success: boolean }>("/archive", {
    method: "POST",
    body: JSON.stringify({ imageId, action: "restore" }),
  });
}

export async function deleteArchive(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/archive/${id}`, {
    method: "DELETE",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Delete archive error ${res.status}`);
}

export async function archiveGroup(group: string, targetStorage?: string) {
  const res = await fetch(`${API_BASE}/api/v1/archive`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify({ group, targetStorage: targetStorage || "s3" }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Archive error ${res.status}`);
  }
  return res.json();
}

// ─── Trivy Scan ────────────────────────────────────────────

export interface Vulnerability {
  id: string;
  severity: "CRITICAL" | "HIGH" | "MEDIUM" | "LOW";
  package: string;
  installedVersion: string;
  fixedVersion: string;
  title: string;
}

export interface ScanResult {
  vulnerabilities: Vulnerability[];
  scannedAt: string;
  totalCritical: number;
  totalHigh: number;
  totalMedium: number;
  totalLow: number;
}

export async function runTrivyScan(image: { repo: string; tag: string }): Promise<ScanResult> {
  return request("/trivy-scan", {
    method: "POST",
    body: JSON.stringify({ image: `${image.repo}:${image.tag}` }),
  });
}

// ─── Push (DockerHub → Registry) ───────────────────────────

export interface PushResult {
  success: boolean;
  imageId: string;
  digest: string;
  blobsCopied: number;
  blobsSkipped: number;
  totalBytes: number;
  logs: string[];
}

export async function pushToRegistry(opts: {
  image: string;
  targetRegistry: string;
  targetRepo?: string;
  group?: string;
}): Promise<PushResult> {
  const res = await fetch(`${API_BASE}/api/v1/push`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify(opts),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Push error ${res.status}`);
  }
  return res.json();
}

// ─── Export (registry-to-registry) ─────────────────────────

export async function exportImage(opts: {
  imageId: string;
  sourceRegistry: string;
  targetRegistry: string;
  repo: string;
  tag: string;
}) {
  return request<{ success: boolean; message: string }>("/registry-images", {
    method: "POST",
    body: JSON.stringify({ ...opts, action: "export" }),
  });
}

// ─── Daemon Status ─────────────────────────────────────────────

export interface DaemonStatus {
  status: string;      // "running" | "stopped"
  running: boolean;
  workers: number;
  lastRun: string | null;
  nextRun: string | null;
  pid?: number;
  startedAt?: string | null;
}

const BACKEND_URL = import.meta.env.VITE_SUPABASE_URL || "";

export async function fetchDaemonStatus(): Promise<DaemonStatus> {
  const res = await fetch(`${BACKEND_URL}/api/v1/daemon/status`, {
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Daemon status error ${res.status}`);
  const data = await res.json();
  return { ...data, running: data.status === "running" };
}

export async function controlDaemon(action: "start" | "stop"): Promise<DaemonStatus> {
  const res = await fetch(`${BACKEND_URL}/api/v1/daemon/${action}`, {
    method: "POST",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Daemon ${action} error ${res.status}`);
  const data = await res.json();
  return { ...data, running: data.status === "running" };
}

// ─── Config / Registries ───────────────────────────────────

export interface RegistryConfig {
  id: string;
  name: string;
  registryType: string;  // "ocir" | "ecr" | "dockerhub"
  endpoint: string;
  region: string;
  tenancy: string;
  credentialSource: string;
  createdAt: string;
}

const API_URL = import.meta.env.VITE_SUPABASE_URL || "";

export async function fetchRegistries(): Promise<{ registries: RegistryConfig[]; total: number }> {
  const res = await fetch(`${API_URL}/api/v1/registries`, { headers: headers() });
  if (!res.ok) throw new Error(`Registries error ${res.status}`);
  return res.json();
}

export async function saveRegistry(reg: {
  name: string;
  registryType: string;
  endpoint: string;
  region: string;
  tenancy?: string;
  authUsername?: string;
  authToken?: string;
  authExtra?: string;
}): Promise<RegistryConfig> {
  const res = await fetch(`${API_URL}/api/v1/registries`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify(reg),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Save error ${res.status}`);
  }
  return res.json();
}

export async function deleteRegistry(id: string): Promise<void> {
  const res = await fetch(`${API_URL}/api/v1/registries/${id}`, {
    method: "DELETE",
    headers: headers(),
  });
  if (!res.ok) throw new Error(`Delete error ${res.status}`);
}

export async function testRegistry(endpoint: string): Promise<{ endpoint: string; success: boolean; error?: string }> {
  const res = await fetch(`${API_URL}/api/v1/registries/test`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify({ endpoint }),
  });
  if (!res.ok) throw new Error(`Test error ${res.status}`);
  return res.json();
}

// Backward-compatible static list for components that still use it
export const registries: Registry[] = [
  { id: "ocir-fra", name: "OCIR Frankfurt", region: "eu-frankfurt-1" },
  { id: "ecr-use1", name: "ECR us-east-1", region: "us-east-1" },
  { id: "dockerhub", name: "Docker Hub", region: "global" },
];
