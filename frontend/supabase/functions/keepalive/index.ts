import { serve } from "https://deno.land/std@0.168.0/http/server.ts";

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
};

type KeepaliveStrategy = 'pull' | 'retag' | 'native';

interface KeepaliveRequest {
  imageIds?: string[];
  group?: string;
  strategy?: KeepaliveStrategy;
}

interface KeepaliveResult {
  imageId: string;
  repo: string;
  tag: string;
  strategy: KeepaliveStrategy;
  success: boolean;
  newExpiry: string;
  error?: string;
}

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response(null, { headers: corsHeaders });
  }

  if (req.method !== 'POST') {
    return new Response(
      JSON.stringify({ error: 'Method not allowed. Use POST to trigger keepalive.' }),
      { status: 405, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    );
  }

  try {
    const body: KeepaliveRequest = await req.json();
    const { imageIds, group, strategy = 'pull' } = body;

    if (!imageIds?.length && !group) {
      return new Response(
        JSON.stringify({ error: 'Provide imageIds or group to trigger keepalive' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      );
    }

    console.log(`[rgk] Keepalive triggered — strategy: ${strategy}, targets: ${imageIds?.length || group}`);

    // TODO: Implement per-strategy keepalive logic
    //
    // PULL strategy:
    //   docker pull <image> → resets retention timer on most registries
    //   OCIR: oci artifacts container image pull
    //   ECR: aws ecr batch-get-image + put-image
    //
    // RETAG strategy:
    //   docker tag <image> <image>-keepalive-<timestamp>
    //   Then delete the temp tag — the retag resets the clock
    //
    // NATIVE strategy:
    //   OCIR: PUT /20180419/images/{imageId} lifecycle policy
    //   ECR: putLifecyclePolicy with extended retention

    // Simulate processing
    await new Promise(resolve => setTimeout(resolve, 300));

    const mockTargets = [
      { imageId: "1", repo: "platform/api-gateway", tag: "v2.14.3" },
      { imageId: "2", repo: "platform/auth-service", tag: "v1.8.0" },
      { imageId: "3", repo: "platform/worker", tag: "v3.2.1" },
    ];

    const targets = imageIds
      ? mockTargets.filter(t => imageIds.includes(t.imageId))
      : mockTargets;

    const results: KeepaliveResult[] = targets.map(t => ({
      imageId: t.imageId,
      repo: t.repo,
      tag: t.tag,
      strategy,
      success: true,
      newExpiry: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
    }));

    console.log(`[rgk] Keepalive complete — ${results.length} images refreshed`);

    return new Response(
      JSON.stringify({
        timestamp: new Date().toISOString(),
        strategy,
        processed: results.length,
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
