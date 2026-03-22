package registry

import "fmt"

// CopyResult holds the outcome of a cross-registry image copy.
type CopyResult struct {
	Digest       string
	BlobsCopied  int
	BlobsSkipped int
	TotalBytes   int64
}

// CopyDockerHubToECR pulls an image from Docker Hub and pushes it to ECR.
func CopyDockerHubToECR(src *DockerHubAdapter, dst *ECRAdapter, srcRepo, srcTag, dstRepo string, progress func(string)) (CopyResult, error) {
	var result CopyResult

	// Step 1: Pull manifest from DockerHub
	progress("Pulling manifest from Docker Hub...")
	manifest, err := src.PullManifest(srcRepo, srcTag)
	if err != nil {
		return result, fmt.Errorf("pull manifest: %w", err)
	}

	// Step 2: Parse manifest to get blob descriptors
	config, layers, err := ParseManifestDescriptors(manifest)
	if err != nil {
		return result, fmt.Errorf("parse manifest: %w", err)
	}
	progress(fmt.Sprintf("Manifest has %d layers + 1 config", len(layers)))

	// Step 3: Create ECR repository (idempotent)
	progress(fmt.Sprintf("Ensuring ECR repository '%s' exists...", dstRepo))
	if err := dst.CreateRepository(dstRepo); err != nil {
		return result, fmt.Errorf("create repository: %w", err)
	}

	// Step 4: Copy all blobs (config + layers)
	allBlobs := append([]BlobDescriptor{config}, layers...)
	for i, blob := range allBlobs {
		label := "config"
		if i > 0 {
			label = fmt.Sprintf("layer %d/%d", i, len(layers))
		}

		// Check if blob already exists in ECR
		exists, err := dst.CheckBlobExists(dstRepo, blob.Digest)
		if err != nil {
			return result, fmt.Errorf("check blob %s: %w", label, err)
		}
		if exists {
			progress(fmt.Sprintf("  %s: %s (already exists, skipped)", label, blob.Digest[:19]))
			result.BlobsSkipped++
			continue
		}

		// Pull blob from DockerHub
		reader, size, err := src.PullBlob(srcRepo, blob.Digest)
		if err != nil {
			return result, fmt.Errorf("pull %s: %w", label, err)
		}

		// Push blob to ECR
		progress(fmt.Sprintf("  %s: %s (%d bytes)", label, blob.Digest[:19], blob.Size))
		if err := dst.PushBlob(dstRepo, blob.Digest, size, reader); err != nil {
			reader.Close()
			return result, fmt.Errorf("push %s: %w", label, err)
		}
		reader.Close()

		result.BlobsCopied++
		result.TotalBytes += blob.Size
	}

	// Step 5: Push manifest to ECR
	progress("Pushing manifest to ECR...")
	digest, err := dst.PushManifest(dstRepo, srcTag, manifest)
	if err != nil {
		return result, fmt.Errorf("push manifest: %w", err)
	}
	if digest == "" {
		digest = manifest.Digest
	}
	result.Digest = digest

	return result, nil
}
