# Static File Serving

## Current Approach

Ourocodus uses Go's standard `http.FileServer` to serve static files for the PWA frontend.

**Implementation:** `cmd/relay/main.go`
- Serves files from `web/` directory
- Simple, zero-dependency solution
- Suitable for development and small-scale deployments

**Characteristics:**
- No caching headers (browser default caching)
- No compression (relies on reverse proxy if needed)
- Single-instance serving (no load balancing built-in)

## When to Scale

The current approach is sufficient for:
- Development environments
- Internal tools with <100 concurrent users
- Single-instance deployments

## Future Scalability Options

When traffic increases or global distribution is needed:

### 1. CDN (Recommended for production)
- Serve static assets from Cloudflare, Fastly, or AWS CloudFront
- Keep relay for WebSocket connections only
- Benefits: global distribution, caching, DDoS protection

### 2. Separate Static Server
- nginx or Apache for static files
- Relay handles only WebSocket and API
- Benefits: better caching control, compression, tuned configs

### 3. Object Storage
- S3, GCS, or similar with public access
- Direct browser access to assets
- Benefits: infinite scalability, no server load

## Decision Timeline

- **Now:** http.FileServer is appropriate
- **Milestone 3-4:** Evaluate if traffic patterns require change
- **Production:** Implement CDN before public launch

## References

- Implementation: `cmd/relay/main.go`
- Related discussion: Issue #49
