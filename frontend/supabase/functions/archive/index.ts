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
  const action = url.searchParams.get('action'); // 'archive' | 'restore' | 'list'

  try {
    // GET = list archived images
    if (req.method === 'GET' || action === 'list') {
      // TODO: Query actual object storage for archived manifests
      // OCI: oci os object list --bucket-name regikeep-archive
      // S3: aws s3api list-objects-v2 --bucket regikeep-archive-s3

      const archived = [
        { id: "1", repo: "platform/api-gateway", tag: "v2.12.0", compressedSize: "89 MB", originalSize: "245 MB", archivedAt: "2026-02-15T10:00:00Z", restorable: true, storageBackend: "oci-os" },
        { id: "2", repo: "ml/inference-engine", tag: "v0.8.0", compressedSize: "420 MB", originalSize: "1.1 GB", archivedAt: "2026-02-01T14:00:00Z", restorable: true, storageBackend: "s3" },
        { id: "3", repo: "frontend/web-app", tag: "v3.9.2", compressedSize: "52 MB", originalSize: "148 MB", archivedAt: "2026-01-20T08:00:00Z", restorable: true, storageBackend: "oci-os" },
        { id: "4", repo: "tools/ci-runner", tag: "v0.9.0", compressedSize: "150 MB", originalSize: "410 MB", archivedAt: "2026-01-10T12:00:00Z", restorable: false, storageBackend: "s3" },
      ];

      const totalOriginal = 2053; // MB
      const totalCompressed = 711; // MB
      const savings = totalOriginal - totalCompressed;

      return new Response(
        JSON.stringify({
          images: archived,
          storage: {
            totalOriginalMB: totalOriginal,
            totalCompressedMB: totalCompressed,
            savingsMB: savings,
            savingsPercent: Math.round((savings / totalOriginal) * 100),
          },
        }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      );
    }

    // POST = archive or restore
    if (req.method === 'POST') {
      const body = await req.json();

      if (action === 'archive') {
        const { imageIds, group, targetStorage = 'oci-os' } = body;

        if (!imageIds?.length && !group) {
          return new Response(
            JSON.stringify({ error: 'Provide imageIds or group to archive' }),
            { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
          );
        }

        console.log(`[rgk] Archive started — target: ${targetStorage}, images: ${imageIds?.length || group}`);

        // TODO: Implement archive pipeline
        // Step 1: Pull image layers (docker save / skopeo copy)
        // Step 2: Deduplicate layers across images
        // Step 3: Compress with zstd
        // Step 4: Upload to object storage
        //   OCI: oci os object put --bucket-name ... --file ...
        //   S3: aws s3 cp ... s3://bucket/...
        // Step 5: Verify integrity (checksum match)
        // Step 6: Optionally delete from source registry

        await new Promise(resolve => setTimeout(resolve, 500));

        return new Response(
          JSON.stringify({
            success: true,
            action: 'archive',
            targetStorage,
            timestamp: new Date().toISOString(),
            steps: [
              { step: 'pull', status: 'complete', duration: '2.3s' },
              { step: 'dedup', status: 'complete', duration: '0.8s', layersDeduped: 4 },
              { step: 'compress', status: 'complete', duration: '5.1s', ratio: '65%' },
              { step: 'upload', status: 'complete', duration: '3.2s' },
              { step: 'verify', status: 'complete', duration: '0.4s', checksumMatch: true },
            ],
          }),
          { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
        );
      }

      if (action === 'restore') {
        const { archiveId, targetRegistry } = body;

        if (!archiveId || !targetRegistry) {
          return new Response(
            JSON.stringify({ error: 'Provide archiveId and targetRegistry' }),
            { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
          );
        }

        console.log(`[rgk] Restore started — archive: ${archiveId}, target: ${targetRegistry}`);

        // TODO: Implement restore pipeline
        // Step 1: Download from object storage
        // Step 2: Decompress
        // Step 3: Push layers to target registry
        // Step 4: Push manifest
        // Step 5: Verify image is pullable

        await new Promise(resolve => setTimeout(resolve, 500));

        return new Response(
          JSON.stringify({
            success: true,
            action: 'restore',
            archiveId,
            targetRegistry,
            timestamp: new Date().toISOString(),
            steps: [
              { step: 'download', status: 'complete', duration: '2.8s' },
              { step: 'decompress', status: 'complete', duration: '3.5s' },
              { step: 'push_layers', status: 'complete', duration: '6.2s', layersPushed: 7 },
              { step: 'push_manifest', status: 'complete', duration: '0.3s' },
              { step: 'verify', status: 'complete', duration: '1.1s', pullable: true },
            ],
          }),
          { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
        );
      }

      return new Response(
        JSON.stringify({ error: 'Invalid action. Use ?action=archive or ?action=restore' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
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
