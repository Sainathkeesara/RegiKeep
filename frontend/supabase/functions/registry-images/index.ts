import { serve } from "https://deno.land/std@0.168.0/http/server.ts";

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
};

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response(null, { headers: corsHeaders });
  }

  const url = new URL(req.url);
  const registry = url.searchParams.get('registry') || 'all';
  const status = url.searchParams.get('status');
  const search = url.searchParams.get('search');

  try {
    // POST = pin/unpin action
    if (req.method === 'POST') {
      const { imageId, action } = await req.json();

      if (!imageId || !['pin', 'unpin'].includes(action)) {
        return new Response(
          JSON.stringify({ error: 'Invalid request. Requires imageId and action (pin|unpin)' }),
          { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
        );
      }

      // TODO: Replace with actual registry API calls
      // OCIR: PUT /20180419/images/{imageId} with { isPinned: true }
      // ECR: putImageTagMutability / putLifecyclePolicy
      // Docker Hub: tag manipulation via API

      console.log(`[rgk] ${action.toUpperCase()} image ${imageId}`);

      return new Response(
        JSON.stringify({
          success: true,
          imageId,
          action,
          pinned: action === 'pin',
          timestamp: new Date().toISOString(),
        }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      );
    }

    // GET = list images with optional filters
    if (req.method === 'GET') {
      // TODO: Replace with actual registry API calls per provider
      // OCIR: GET /20180419/images?compartmentId=...
      // ECR: ecr.describeImages({ repositoryName })
      // Docker Hub: GET /v2/repositories/{namespace}/{repo}/tags

      const mockImages = [
        { id: "1", repo: "platform/api-gateway", tag: "v2.14.3", digest: "sha256:a1b2c3d4e5f6", region: "eu-frankfurt-1", size: "245 MB", group: "production", pinned: true, expiresIn: 30, lastKeepalive: new Date().toISOString(), status: "safe", registry: "ocir-fra" },
        { id: "2", repo: "platform/auth-service", tag: "v1.8.0", digest: "sha256:b2c3d4e5f6a7", region: "eu-frankfurt-1", size: "189 MB", group: "production", pinned: true, expiresIn: 28, lastKeepalive: new Date().toISOString(), status: "safe", registry: "ocir-fra" },
        { id: "3", repo: "platform/worker", tag: "v3.2.1", digest: "sha256:c3d4e5f6a7b8", region: "eu-frankfurt-1", size: "312 MB", group: "staging", pinned: true, expiresIn: 5, lastKeepalive: new Date().toISOString(), status: "warning", registry: "ocir-fra" },
        { id: "4", repo: "ml/inference-engine", tag: "v0.9.4", digest: "sha256:d4e5f6a7b8c9", region: "us-east-1", size: "1.2 GB", group: "ml-models", pinned: false, expiresIn: 1, lastKeepalive: new Date().toISOString(), status: "critical", registry: "ecr-use1" },
      ];

      let filtered = mockImages;

      if (registry !== 'all') {
        filtered = filtered.filter(img => img.registry === registry);
      }
      if (status) {
        filtered = filtered.filter(img => img.status === status);
      }
      if (search) {
        const q = search.toLowerCase();
        filtered = filtered.filter(img =>
          img.repo.toLowerCase().includes(q) || img.tag.toLowerCase().includes(q)
        );
      }

      return new Response(
        JSON.stringify({ images: filtered, total: filtered.length }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      );
    }

    return new Response(
      JSON.stringify({ error: 'Method not allowed' }),
      { status: 405, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  }
});
