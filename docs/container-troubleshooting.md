# Container Troubleshooting Guide

## Claude Code Container Issues

### Container fails to start

**Symptom:** Container exits immediately after starting

**Check:**
1. Verify credentials are mounted: `docker inspect <container> | grep Mounts`
2. Check entry script logs: `docker logs <container>`
3. Verify .env format: `cat .creds/.env` should have `ANTHROPIC_API_KEY=sk-...`

### Health check failing

**Symptom:** Container shows "unhealthy" status

**Check:**
1. Verify PID file exists: `docker exec <container> cat /tmp/claude-code.pid`
2. Check if process is running: `docker exec <container> ps aux | grep claude-code`
3. Check logs for errors: `docker logs <container>`

### Permission denied errors

**Symptom:** "Permission denied" when writing to /tmp or /workspace

**Check:**
1. Verify tmpfs is mounted: `docker inspect <container> | grep Tmpfs`
2. Check if running as node user: `docker exec <container> whoami` (should return "node")
3. Verify workspace permissions: `docker exec <container> ls -la /workspace`

### API key not found

**Symptom:** "ANTHROPIC_API_KEY not set" error

**Check:**
1. Verify .creds/.env exists: `ls -la <workspace>/.creds/`
2. Check mount: `docker inspect <container> | grep -A5 .creds`
3. Verify format: File must have `ANTHROPIC_API_KEY=sk-...` format

## Runtime Hardening

The container runs with these security options by default:
- `--read-only`: Root filesystem is read-only
- `--cap-drop=ALL`: All Linux capabilities dropped
- `--security-opt=no-new-privileges`: Cannot gain privileges
- `--tmpfs /tmp`: Writable tmpfs for temporary files (256MB, noexec, nosuid)
- `--memory=2g`: 2GB memory limit
- `--cpus=2`: 2 CPU cores limit

### Configurable Resource Limits

Resource limits can be overridden via environment variables on the host:

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_MEMORY_LIMIT_MB` | 2048 | Memory limit in megabytes |
| `AGENT_CPU_LIMIT` | 2.0 | CPU cores (fractional allowed) |
| `AGENT_TMPFS_SIZE_MB` | 256 | tmpfs size for /tmp in megabytes |

Example:
```bash
export AGENT_MEMORY_LIMIT_MB=4096
export AGENT_CPU_LIMIT=4.0
export AGENT_TMPFS_SIZE_MB=512
# Then start your agent orchestrator
```

## Security Considerations

### API Key Exposure via Environment

**Warning:** When credentials are sourced from `.creds/.env`, the `ANTHROPIC_API_KEY` is exposed in the process environment. This is visible via `/proc/$pid/environ` to anyone with `docker exec` access to the container.

**Mitigations applied:**
- Read-only root filesystem prevents persistent modifications
- All Linux capabilities dropped (`--cap-drop=ALL`)
- No privilege escalation (`--security-opt=no-new-privileges`)
- Credentials file mounted read-only with 0600 permissions
- Container runs as non-root user (`node`, UID 1000)

**Recommendations:**
- Restrict Docker socket access to trusted users only
- Use network policies to limit container egress
- Consider migrating to Docker secrets or file-descriptor passing in production
- Monitor container exec events in production environments

### User and Home Directory

The container uses the `node` user (UID 1000) from the Node.js base image:
- Home directory: `/home/node`
- Credential paths: `/home/node/.creds/`, `/home/node/.claude/`
- SSH keys: `/home/node/.ssh/`

This differs from some documentation that may reference `/home/agent`. The UID (1000) is consistent with container best practices for non-root execution.

## Building Images

```bash
# Build base image first
docker build -f Dockerfile.claude-code-base -t ourocodus/claude-code-base:latest .

# Then build agent image
docker build -f Dockerfile.agent -t ourocodus/agent:latest .
```

## Testing Container Manually

```bash
# Run container with test credentials
mkdir -p /tmp/test-creds
echo "ANTHROPIC_API_KEY=sk-test-key" > /tmp/test-creds/.env

docker run --rm -it \
  --read-only \
  --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --tmpfs /tmp:size=256m,noexec,nosuid \
  -v /tmp/test-creds:/home/node/.creds:ro \
  ourocodus/agent:latest
```

## Kubernetes Deployment

### Example Pod Specification

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: claude-code-agent
  labels:
    app: claude-code
spec:
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    runAsNonRoot: true
  containers:
  - name: agent
    image: ghcr.io/ourocodus/agent:latest
    securityContext:
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
    resources:
      requests:
        memory: "512Mi"
        cpu: "500m"
      limits:
        memory: "2Gi"
        cpu: "2"
    volumeMounts:
    - name: tmp
      mountPath: /tmp
    - name: workspace
      mountPath: /workspace
    - name: credentials
      mountPath: /home/node/.creds
      readOnly: true
    livenessProbe:
      exec:
        command:
        - /usr/local/bin/healthcheck.sh
      initialDelaySeconds: 5
      periodSeconds: 30
    readinessProbe:
      exec:
        command:
        - /usr/local/bin/healthcheck.sh
      initialDelaySeconds: 5
      periodSeconds: 10
  volumes:
  - name: tmp
    emptyDir:
      medium: Memory
      sizeLimit: 256Mi
  - name: workspace
    emptyDir: {}
  - name: credentials
    secret:
      secretName: claude-credentials
      defaultMode: 0400
```

### Creating the Credentials Secret

```bash
kubectl create secret generic claude-credentials \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-... \
  --dry-run=client -o yaml > claude-credentials.yaml

# Apply the secret
kubectl apply -f claude-credentials.yaml
```

### Security Notes

- `runAsNonRoot: true` enforces non-root execution at the Pod level
- `readOnlyRootFilesystem: true` prevents filesystem modifications
- `allowPrivilegeEscalation: false` blocks setuid/setgid
- `capabilities.drop: ALL` removes all Linux capabilities
- `emptyDir` with `medium: Memory` creates tmpfs for /tmp
- Credentials mounted with `0400` permissions for security
