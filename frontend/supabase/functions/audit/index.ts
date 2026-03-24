import { serve } from "https://deno.land/std@0.168.0/http/server.ts";

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
};

interface AuditResult {
  imageId: string;
  repo: string;
  tag: string;
  region: string;
  expiresIn: number;
  status: string;
  risk: 'critical' | 'warning' | 'unpinned';
  recommendation: string;
}

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response(null, { headers: corsHeaders });
  }

  if (req.method !== 'POST') {
    return new Response(
      JSON.stringify({ error: 'Method not allowed. Use POST to run audit.' }),
      { status: 405, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  }

  try {
    const { dryRun = true, registryFilter } = await req.json().catch(() => ({ dryRun: true }));

    console.log(`[rgk] Audit started — dryRun: ${dryRun}, registry: ${registryFilter || 'all'}`);

    // TODO: Replace with actual registry retention policy checks
    // OCIR: GET /20180419/retentionPolicies + compare image ages
    // ECR: getLifecyclePolicy + evaluateLifecyclePolicy
    // Docker Hub: check tag age vs retention rules

    // Simulate scanning delay
    await new Promise(resolve => setTimeout(resolve, 500));

    const atRiskImages: AuditResult[] = [
      {
        imageId: "4",
        repo: "ml/inference-engine",
        tag: "v0.9.4",
        region: "us-east-1",
        expiresIn: 1,
        status: "critical",
        risk: "critical",
        recommendation: "Pin immediately or archive to cold storage",
      },
      {
        imageId: "9",
        repo: "platform/metrics-collector",
        tag: "v2.1.0",
        region: "us-east-1",
        expiresIn: 2,
        status: "critical",
        risk: "critical",
        recommendation: "Pin immediately — used by monitoring stack",
      },
      {
        imageId: "3",
        repo: "platform/worker",
        tag: "v3.2.1",
        region: "eu-frankfurt-1",
        expiresIn: 5,
        status: "warning",
        risk: "warning",
        recommendation: "Schedule keepalive or pin before expiry",
      },
      {
        imageId: "6",
        repo: "frontend/web-app",
        tag: "v4.1.0",
        region: "global",
        expiresIn: 3,
        status: "warning",
        risk: "warning",
        recommendation: "Review retention policy for production images",
      },
      {
        imageId: "7",
        repo: "tools/ci-runner",
        tag: "v1.0.2",
        region: "global",
        expiresIn: -1,
        status: "unpinned",
        risk: "unpinned",
        recommendation: "Unpinned image — will be garbage collected",
      },
      {
        imageId: "10",
        repo: "platform/config-server",
        tag: "v1.5.0",
        region: "eu-frankfurt-1",
        expiresIn: -1,
        status: "unpinned",
        risk: "unpinned",
        recommendation: "Unpinned image — archive or delete",
      },
    ];

    let results = atRiskImages;
    if (registryFilter && registryFilter !== 'all') {
      // Filter would apply based on registry mapping
    }

    const summary = {
      totalScanned: 12,
      atRisk: results.length,
      critical: results.filter(r => r.risk === 'critical').length,
      warning: results.filter(r => r.risk === 'warning').length,
      unpinned: results.filter(r => r.risk === 'unpinned').length,
    };

    console.log(`[rgk] Audit complete — ${summary.atRisk} at risk (${summary.critical} critical)`);

    return new Response(
      JSON.stringify({
        dryRun,
        timestamp: new Date().toISOString(),
        summary,
        results,
      }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  }
});
