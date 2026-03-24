import { useState } from "react";
import { Database, Clock, HardDrive, Bell, Cloud, Server, Container, Loader2, Check, ShieldAlert } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/hooks/use-toast";
import { fetchRegistries, saveRegistry, testRegistry } from "@/lib/api";

function Section({
  icon: Icon,
  title,
  children,
  testStatus,
  onTest,
  testing,
}: {
  icon: React.ElementType;
  title: string;
  children: React.ReactNode;
  testStatus?: "idle" | "testing" | "ok" | "fail";
  onTest?: () => void;
  testing?: boolean;
}) {
  return (
    <div className="border border-border rounded-md bg-card">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
        <Icon className="h-4 w-4 text-primary" />
        <h2 className="text-sm font-mono-data font-semibold text-foreground">{title}</h2>
        <div className="ml-auto flex items-center gap-2">
          {testStatus === "ok" && <Check className="h-3.5 w-3.5 text-green-500" />}
          {testStatus === "fail" && <ShieldAlert className="h-3.5 w-3.5 text-red-500" />}
          {onTest && (
            <Button
              variant="outline"
              size="sm"
              className="h-6 px-2 text-[10px] font-mono-data"
              onClick={onTest}
              disabled={testing}
            >
              {testing ? <Loader2 className="h-3 w-3 animate-spin mr-1" /> : null}
              Test
            </Button>
          )}
        </div>
      </div>
      <div className="p-4 space-y-3">{children}</div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <div className="space-y-1">
      <Label className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">{label}</Label>
      <Input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="h-8 text-xs font-mono-data bg-muted border-border"
      />
    </div>
  );
}

interface RegistryForm {
  name: string;
  endpoint: string;
  region: string;
  tenancy: string;
  authUsername: string;
  authToken: string;
  authExtra: string;
}

const EMPTY: RegistryForm = { name: "", endpoint: "", region: "", tenancy: "", authUsername: "", authToken: "", authExtra: "" };

export default function SettingsPage() {
  const queryClient = useQueryClient();

  // Load saved registries from backend
  const { data: regData } = useQuery({
    queryKey: ["registries"],
    queryFn: fetchRegistries,
  });

  // Registry forms
  const [ocir, setOcir] = useState<RegistryForm>({ ...EMPTY, name: "ocir" });
  const [ecr, setEcr] = useState<RegistryForm>({ ...EMPTY, name: "ecr" });
  const [dockerhub, setDockerhub] = useState<RegistryForm>({ ...EMPTY, name: "dockerhub", endpoint: "docker.io" });

  // Storage forms (env-only, informational)
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3Region, setS3Region] = useState("us-east-1");
  const [s3Prefix, setS3Prefix] = useState("/archives");
  const [ociBucket, setOciBucket] = useState("");
  const [ociNamespace, setOciNamespace] = useState("");
  const [ociRegion, setOciRegion] = useState("");

  // Daemon (informational)
  const [keepaliveInterval, setKeepaliveInterval] = useState("6h");
  const [concurrency, setConcurrency] = useState("4");
  const [autoStart, setAutoStart] = useState(false);

  // Test states per registry type
  const [testStates, setTestStates] = useState<Record<string, "idle" | "testing" | "ok" | "fail">>({
    ocir: "idle",
    ecr: "idle",
    dockerhub: "idle",
  });

  const savedRegs = regData?.registries ?? [];
  const getExisting = (type: string) => savedRegs.find((r) => r.registryType === type);

  const ocirSaved = getExisting("ocir");
  const ecrSaved = getExisting("ecr");
  const dockerhubSaved = getExisting("dockerhub");

  const handleTest = async (type: string, endpoint: string) => {
    if (!endpoint) {
      toast({ title: "Enter an endpoint first", variant: "destructive" });
      return;
    }
    setTestStates((s) => ({ ...s, [type]: "testing" }));
    try {
      const result = await testRegistry(endpoint);
      setTestStates((s) => ({ ...s, [type]: result.success ? "ok" : "fail" }));
      if (result.success) {
        toast({ title: `${type.toUpperCase()} connection successful` });
      } else {
        toast({ title: `${type.toUpperCase()} test failed`, description: result.error, variant: "destructive" });
      }
    } catch (e) {
      setTestStates((s) => ({ ...s, [type]: "fail" }));
      toast({ title: `${type.toUpperCase()} test failed`, description: (e as Error).message, variant: "destructive" });
    }
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      const toSave: Parameters<typeof saveRegistry>[0][] = [];

      if (ocir.endpoint && !ocirSaved) {
        toSave.push({
          name: ocir.name || "ocir", registryType: "ocir", endpoint: ocir.endpoint,
          region: ocir.region, tenancy: ocir.tenancy,
          authUsername: ocir.authUsername, authToken: ocir.authToken, authExtra: ocir.authExtra,
        });
      }
      if (ecr.endpoint && !ecrSaved) {
        toSave.push({
          name: ecr.name || "ecr", registryType: "ecr", endpoint: ecr.endpoint,
          region: ecr.region,
          authUsername: ecr.authUsername, authToken: ecr.authToken, authExtra: ecr.authExtra,
        });
      }
      if (dockerhub.endpoint && !dockerhubSaved) {
        toSave.push({
          name: dockerhub.name || "dockerhub", registryType: "dockerhub", endpoint: dockerhub.endpoint,
          region: "",
          authUsername: dockerhub.authUsername, authToken: dockerhub.authToken,
        });
      }

      for (const reg of toSave) {
        await saveRegistry(reg);
      }
      return toSave.length;
    },
    onSuccess: (count) => {
      queryClient.invalidateQueries({ queryKey: ["registries"] });
      toast({
        title: "Configuration saved",
        description: count > 0 ? `${count} new registr${count === 1 ? "y" : "ies"} added.` : "Settings updated.",
      });
    },
    onError: (e) => {
      toast({
        title: "Save failed",
        description: (e as Error).message,
        variant: "destructive",
      });
    },
  });

  const updateOcir = (field: keyof RegistryForm, val: string) => setOcir((s) => ({ ...s, [field]: val }));
  const updateEcr = (field: keyof RegistryForm, val: string) => setEcr((s) => ({ ...s, [field]: val }));
  const updateDockerhub = (field: keyof RegistryForm, val: string) => setDockerhub((s) => ({ ...s, [field]: val }));

  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold font-mono-data tracking-tight text-foreground">Settings</h1>

      {savedRegs.length > 0 && (
        <div className="text-xs font-mono-data text-muted-foreground border border-border rounded-md p-3 bg-card">
          <span className="text-foreground font-semibold">{savedRegs.length}</span> registr{savedRegs.length === 1 ? "y" : "ies"} configured:{" "}
          {savedRegs.map((r) => (
            <span key={r.id} className="inline-flex items-center gap-1 mr-3">
              <span className="text-primary">{r.name}</span>
              <span className="text-muted-foreground">({r.registryType})</span>
            </span>
          ))}
        </div>
      )}

      <div className="grid grid-cols-2 gap-4">
        {/* OCIR */}
        <Section
          icon={Database}
          title="Oracle OCIR"
          testStatus={testStates.ocir}
          testing={testStates.ocir === "testing"}
          onTest={() => handleTest("ocir", ocir.endpoint || ocirSaved?.endpoint || "")}
        >
          <Field label="Endpoint" value={ocir.endpoint} onChange={(v) => updateOcir("endpoint", v)} placeholder={ocirSaved?.endpoint || "fra.ocir.io"} />
          <Field label="Tenancy Namespace" value={ocir.tenancy} onChange={(v) => updateOcir("tenancy", v)} placeholder={ocirSaved?.tenancy || "mytenancy"} />
          <Field label="Region" value={ocir.region} onChange={(v) => updateOcir("region", v)} placeholder={ocirSaved?.region || "eu-frankfurt-1"} />
          <Field label="Username" value={ocir.authUsername} onChange={(v) => updateOcir("authUsername", v)} placeholder="tenancy/oracleidentitycloudservice/user@example.com" />
          <Field label="Auth Token" value={ocir.authToken} onChange={(v) => updateOcir("authToken", v)} type="password" placeholder="Enter auth token" />
          <Field label="Compartment OCID" value={ocir.authExtra} onChange={(v) => updateOcir("authExtra", v)} placeholder="ocid1.compartment.oc1..aaaaaa..." />
          <Field label="Name" value={ocir.name} onChange={(v) => updateOcir("name", v)} placeholder={ocirSaved?.name || "ocir"} />
        </Section>

        {/* ECR */}
        <Section
          icon={Server}
          title="AWS ECR"
          testStatus={testStates.ecr}
          testing={testStates.ecr === "testing"}
          onTest={() => handleTest("ecr", ecr.endpoint || ecrSaved?.endpoint || "")}
        >
          <Field label="ECR Endpoint" value={ecr.endpoint} onChange={(v) => updateEcr("endpoint", v)} placeholder={ecrSaved?.endpoint || "123456789.dkr.ecr.us-east-1.amazonaws.com"} />
          <Field label="Region" value={ecr.region} onChange={(v) => updateEcr("region", v)} placeholder={ecrSaved?.region || "us-east-1"} />
          <Field label="AWS Access Key ID" value={ecr.authUsername} onChange={(v) => updateEcr("authUsername", v)} placeholder="AKIA..." />
          <Field label="AWS Secret Access Key" value={ecr.authToken} onChange={(v) => updateEcr("authToken", v)} type="password" placeholder="Enter secret key" />
          <Field label="Account ID" value={ecr.authExtra} onChange={(v) => updateEcr("authExtra", v)} placeholder="123456789012" />
          <Field label="Name" value={ecr.name} onChange={(v) => updateEcr("name", v)} placeholder={ecrSaved?.name || "ecr"} />
        </Section>

        {/* Docker Hub */}
        <Section
          icon={Container}
          title="Docker Hub"
          testStatus={testStates.dockerhub}
          testing={testStates.dockerhub === "testing"}
          onTest={() => handleTest("dockerhub", dockerhub.endpoint || dockerhubSaved?.endpoint || "")}
        >
          <Field label="Endpoint" value={dockerhub.endpoint} onChange={(v) => updateDockerhub("endpoint", v)} placeholder={dockerhubSaved?.endpoint || "docker.io"} />
          <Field label="Username" value={dockerhub.authUsername} onChange={(v) => updateDockerhub("authUsername", v)} placeholder="myuser" />
          <Field label="Access Token" value={dockerhub.authToken} onChange={(v) => updateDockerhub("authToken", v)} type="password" placeholder="dckr_pat_..." />
          <Field label="Name" value={dockerhub.name} onChange={(v) => updateDockerhub("name", v)} placeholder={dockerhubSaved?.name || "dockerhub"} />
        </Section>

        {/* AWS S3 Storage */}
        <Section icon={Cloud} title="AWS S3 Storage">
          <Field label="Bucket Name" value={s3Bucket} onChange={setS3Bucket} placeholder="regikeep-archive-s3" />
          <Field label="Region" value={s3Region} onChange={setS3Region} placeholder="us-east-1" />
          <Field label="Path Prefix" value={s3Prefix} onChange={setS3Prefix} placeholder="/archives" />
          <p className="text-[10px] text-muted-foreground font-mono-data">Credentials via env: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY</p>
        </Section>

        {/* OCI Object Storage */}
        <Section icon={HardDrive} title="OCI Object Storage">
          <Field label="Bucket Name" value={ociBucket} onChange={setOciBucket} placeholder="regikeep-archive" />
          <Field label="Namespace" value={ociNamespace} onChange={setOciNamespace} placeholder="mytenancy" />
          <Field label="Region" value={ociRegion} onChange={setOciRegion} placeholder="eu-frankfurt-1" />
          <p className="text-[10px] text-muted-foreground font-mono-data">Configured via OCI CLI config profile</p>
        </Section>

        {/* Daemon Configuration */}
        <Section icon={Clock} title="Daemon Configuration">
          <Field label="Keepalive Interval" value={keepaliveInterval} onChange={setKeepaliveInterval} placeholder="6h" />
          <Field label="Concurrency" value={concurrency} onChange={setConcurrency} placeholder="4" />
          <div className="flex items-center justify-between">
            <Label className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Auto-start on boot</Label>
            <Switch checked={autoStart} onCheckedChange={setAutoStart} className="scale-75" />
          </div>
        </Section>

        {/* Alerts & Notifications */}
        <Section icon={Bell} title="Alerts & Notifications">
          <Field label="Failure Threshold" value="" onChange={() => {}} placeholder="3 consecutive failures" />
          <Field label="Notification Channel" value="" onChange={() => {}} placeholder="webhook" />
          <Field label="Webhook URL" value="" onChange={() => {}} placeholder="https://hooks.slack.com/services/xxx" />
          <div className="flex items-center justify-between">
            <Label className="text-[10px] font-mono-data uppercase tracking-wider text-muted-foreground">Email alerts</Label>
            <Switch className="scale-75" />
          </div>
        </Section>
      </div>

      <div className="flex justify-end">
        <Button
          size="sm"
          className="h-8 text-xs font-mono-data"
          onClick={() => saveMutation.mutate()}
          disabled={saveMutation.isPending}
        >
          {saveMutation.isPending ? <Loader2 className="h-3 w-3 animate-spin mr-1" /> : null}
          Save Configuration
        </Button>
      </div>
    </div>
  );
}
