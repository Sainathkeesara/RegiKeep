import { serve } from "https://deno.land/std@0.168.0/http/server.ts";

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type, x-supabase-client-platform, x-supabase-client-platform-version, x-supabase-client-runtime, x-supabase-client-runtime-version',
};

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response(null, { headers: corsHeaders });
  }

  try {
    const { imageId, repo, tag } = await req.json();

    if (!repo || !tag) {
      return new Response(
        JSON.stringify({ error: 'repo and tag are required' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      );
    }

    // TODO: Replace with actual Trivy server API call
    // Example: POST http://<trivy-server>:4954/v1/scan
    // Body: { "image": "repo:tag" }
    const TRIVY_SERVER_URL = Deno.env.get('TRIVY_SERVER_URL');

    if (TRIVY_SERVER_URL) {
      // Real Trivy scan
      const scanRes = await fetch(`${TRIVY_SERVER_URL}/v1/scan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ image: `${repo}:${tag}` }),
      });

      if (!scanRes.ok) {
        const errorBody = await scanRes.text();
        throw new Error(`Trivy scan failed [${scanRes.status}]: ${errorBody}`);
      }

      const scanData = await scanRes.json();
      return new Response(
        JSON.stringify(scanData),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      );
    }

    // Mock response when no Trivy server configured
    console.log(`[rgk] Trivy scan requested for ${repo}:${tag} (imageId: ${imageId}) — returning mock data`);

    const mockResult = {
      scannedAt: new Date().toISOString(),
      image: `${repo}:${tag}`,
      totalCritical: 2,
      totalHigh: 5,
      totalMedium: 12,
      totalLow: 8,
      vulnerabilities: [
        { id: "CVE-2026-1234", severity: "CRITICAL", package: "openssl", installedVersion: "1.1.1t", fixedVersion: "1.1.1u", title: "Buffer overflow in X.509 certificate verification" },
        { id: "CVE-2026-5678", severity: "CRITICAL", package: "glibc", installedVersion: "2.35", fixedVersion: "2.36", title: "Heap-based buffer overflow in nscd" },
        { id: "CVE-2026-9012", severity: "HIGH", package: "curl", installedVersion: "7.88.0", fixedVersion: "7.88.1", title: "HSTS bypass via IDN" },
        { id: "CVE-2026-3456", severity: "HIGH", package: "zlib", installedVersion: "1.2.13", fixedVersion: "1.2.14", title: "Memory corruption in deflate" },
        { id: "CVE-2026-7890", severity: "MEDIUM", package: "libpng", installedVersion: "1.6.39", fixedVersion: "1.6.40", title: "Integer overflow in png_read_chunk" },
      ],
    };

    return new Response(
      JSON.stringify(mockResult),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  }
});
