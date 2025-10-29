# Feature: PostgreSQL Dockerfile and Helm Chart Hardening

## Overview

### Problem Statement
The current PostgreSQL Docker image and Helm chart deployment need hardening to address CVE vulnerabilities and improve production resilience. The primary concerns are:
- Unpinned package versions creating vulnerability exposure
- Unoptimized container image increasing attack surface
- Lack of vulnerability scanning in the build pipeline
- Missing safeguards for persistent state integrity in single-replica deployments

### Goals
1. **Eliminate CVE vulnerabilities** through comprehensive Dockerfile hardening
2. **Ensure persistent state integrity** for single-replica high availability scenarios
3. **Implement security scanning** in CI/CD pipeline to prevent future vulnerabilities
4. **Optimize container image** to reduce size and attack surface

### Target Users
- DevOps engineers deploying PostgreSQL in production Kubernetes clusters
- Security teams requiring compliance and vulnerability management
- Database administrators managing stateful workloads

### Priority
**Critical** - Production blocking issues that must be addressed immediately

---

## Functional Requirements

### FR-1: Version Pinning and Dependency Management

**Description**: All dependencies, base images, and packages must be pinned to specific versions with cryptographic verification to prevent supply chain attacks and ensure reproducible builds.

**User Story**: As a security engineer, I want all container dependencies pinned to specific versions so that I can audit the exact software composition and prevent unexpected vulnerabilities from dependency updates.

**Acceptance Criteria**:
- [ ] Go builder base image pinned to specific version with SHA256 digest (e.g., `golang:1.23.4-bookworm@sha256:...`)
- [ ] Debian base image pinned to specific version with SHA256 digest (e.g., `debian:bookworm-20240130-slim@sha256:...`)
- [ ] All PostgreSQL packages (14-17) pinned to specific versions from apt repository
- [ ] Essential packages (gosu, jq, curl, etc.) pinned to specific versions
- [ ] GPG key for PostgreSQL repository verified with known good fingerprint
- [ ] Go dependencies locked with go.mod/go.sum verification
- [ ] Build fails if any pinned version is unavailable (fail-fast behavior)
- [ ] Documentation includes process for updating pinned versions

**Current State Analysis**:
```dockerfile
# Current - unpinned versions
FROM golang:1.25-bookworm AS pgconfig-builder
FROM debian:bookworm-slim
apt-get install -y postgresql-14 postgresql-15 ...

# Target - pinned versions
FROM golang:1.23.4-bookworm@sha256:abc123... AS pgconfig-builder
FROM debian:bookworm-20240130-slim@sha256:def456...
apt-get install -y postgresql-14=14.10-1.pgdg120+1 ...
```

---

### FR-2: Image Optimization and Attack Surface Reduction

**Description**: Minimize the container image size and reduce the attack surface by removing unnecessary packages, combining layers, and using minimal base images where appropriate.

**User Story**: As a DevOps engineer, I want a smaller, more secure container image so that deployment times are faster, storage costs are lower, and the attack surface is minimized.

**Acceptance Criteria**:
- [ ] Combined RUN commands to reduce layer count by at least 30%
- [ ] Removed build-time dependencies from final image (ca-certificates, wget, gnupg only needed during setup)
- [ ] Multi-stage build artifacts cleaned up (no intermediate files in final image)
- [ ] Final image size reduced by at least 15% from current baseline
- [ ] Evaluated distroless/minimal base alternatives (document decision if keeping debian-slim)
- [ ] Added .dockerignore file to exclude unnecessary build context
- [ ] Removed unnecessary locale files (keep only en_US.UTF-8)
- [ ] PostgreSQL documentation and man pages removed from final image

**Optimization Targets**:
```dockerfile
# Combine these separate RUN commands
RUN apt-get update && \
    apt-get install -y ca-certificates wget ...
RUN wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc ...
RUN apt-get update && apt-get install -y postgresql-14 ...

# Into single optimized layer
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates wget gnupg && \
    wget ... && \
    apt-get update && apt-get install -y postgresql-14=VERSION ... && \
    apt-get remove -y ca-certificates wget gnupg && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*
```

---

### FR-3: Build Security and Supply Chain Protection

**Description**: Implement security best practices in the build process to prevent supply chain attacks, ensure artifact integrity, and eliminate secrets from container layers.

**User Story**: As a security architect, I want cryptographic verification of all downloaded artifacts and no secrets in container layers so that the build process is secure and auditable.

**Acceptance Criteria**:
- [ ] PostgreSQL APT repository GPG key verified against known good fingerprint (ACCC4CF8)
- [ ] APT packages verified with GPG signatures before installation
- [ ] All downloaded files verified with checksums when available
- [ ] No secrets, credentials, or sensitive data in any container layer
- [ ] Builder stage runs as non-root user where possible
- [ ] .dockerignore excludes sensitive files (.env, credentials, keys)
- [ ] BuildKit secrets mounting used for any sensitive build-time operations
- [ ] Multi-stage build ensures no build tools in final image
- [ ] Reproducible builds - same inputs produce same output hash

**Security Enhancements**:
```dockerfile
# Verify GPG key fingerprint explicitly
RUN wget -qO- https://www.postgresql.org/media/keys/ACCC4CF8.asc | \
    gpg --import && \
    gpg --fingerprint 0xACCC4CF8 | grep "B97B 0AFC AA1A 47F0 44F2  44A0 7FCC 7D46 ACCC 4CF8"

# Use --no-install-recommends to minimize packages
RUN apt-get install -y --no-install-recommends postgresql-14=VERSION
```

---

### FR-4: Vulnerability Scanning and SBOM Generation

**Description**: Integrate automated vulnerability scanning into the CI/CD pipeline and generate Software Bill of Materials (SBOM) for compliance and security tracking.

**User Story**: As a security operator, I want automated vulnerability scanning and SBOM generation so that I can track CVEs, ensure compliance, and respond quickly to security issues.

**Acceptance Criteria**:
- [ ] Trivy or Grype scanner integrated into GitHub Actions workflow
- [ ] Image scanned for vulnerabilities before push to registry
- [ ] Build fails on HIGH or CRITICAL severity CVEs (configurable threshold)
- [ ] SBOM generated in SPDX or CycloneDX format using Syft
- [ ] SBOM attached to container image as attestation
- [ ] Vulnerability scan results published as GitHub Security alerts
- [ ] Scheduled weekly scans of published images in registry
- [ ] Helm chart includes security scanning annotations (`container.apparmor.security.beta.kubernetes.io/*`)
- [ ] Documentation includes process for reviewing and remediating CVEs

**CI/CD Integration**:
```yaml
# .github/workflows/docker-build.yml additions
- name: Scan image with Trivy
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.IMAGE_NAME }}
    severity: HIGH,CRITICAL
    exit-code: 1  # Fail build on findings

- name: Generate SBOM
  uses: anchore/sbom-action@v0
  with:
    image: ${{ env.IMAGE_NAME }}
    format: spdx-json
```

---

### FR-5: Persistent State Protection and Data Integrity

**Description**: Implement safeguards to ensure PostgreSQL data integrity in single-replica deployments, including proper volume handling, fsync configuration, and validation checks.

**User Story**: As a database administrator, I want guaranteed data integrity and persistence so that I never lose data due to misconfiguration or pod failures.

**Acceptance Criteria**:
- [ ] Init container validates PVC is bound and writable before starting PostgreSQL
- [ ] PostgreSQL configured with `fsync=on` and `synchronous_commit=on` for durability
- [ ] Data directory ownership and permissions validated on startup
- [ ] Disk space checked before starting PostgreSQL (minimum 10% free space)
- [ ] WAL (Write-Ahead Log) archiving configuration exposed via Helm values
- [ ] PVC retention policy set to `Retain` by default in StatefulSet
- [ ] Helm values validation ensures valid persistence configuration
- [ ] PreStop hook ensures graceful PostgreSQL shutdown with proper transaction completion
- [ ] Probes tuned to prevent false positives during heavy load (connection timeout handling)

**Current Issues to Address**:
- Line 39-52 in statefulset.yaml: `permissions-fix` init container runs as root unnecessarily
- Use `fsGroup` (already configured line 72) instead of chown init container
- Add data integrity validation in postgres-upgrade init container

**Helm Chart Enhancements**:
```yaml
# values.yaml additions
persistence:
  enabled: true
  storageClass: ""
  accessMode: ReadWriteOnce
  size: 10Gi
  retentionPolicy: Retain  # NEW: prevent accidental deletion
  validation:
    enabled: true  # NEW: validate PVC before startup
    minFreeSpacePercent: 10  # NEW: require minimum free space

# PostgreSQL durability settings
conf:
  fsync: "on"  # NEW: ensure durability
  synchronous_commit: "on"  # NEW: wait for WAL write
  wal_level: "replica"  # NEW: enable WAL archiving
  archive_mode: "on"  # NEW: enable archiving
  archive_command: ""  # NEW: configurable archive command
```

---

### FR-6: Runtime Security Hardening

**Description**: Enhance runtime security contexts, implement read-only filesystems where possible, add seccomp profiles, and remove unnecessary privileges.

**User Story**: As a Kubernetes security administrator, I want minimal runtime privileges and system call restrictions so that a compromised container cannot escalate privileges or attack the host.

**Acceptance Criteria**:
- [ ] Read-only root filesystem enabled with explicit tmpfs mounts for writable directories
- [ ] Seccomp profile created and applied limiting syscalls to PostgreSQL requirements
- [ ] AppArmor/SELinux profiles documented and optionally enabled
- [ ] Remove `permissions-fix` init container that runs as root (use fsGroup instead)
- [ ] PodSecurityStandard labels added for `restricted` profile compatibility
- [ ] Capabilities explicitly limited to minimum required (currently drops ALL - good)
- [ ] No privilege escalation (already configured - maintain)
- [ ] Service account with minimal RBAC permissions
- [ ] Network policies documented (optional implementation)

**Security Context Improvements**:
```yaml
# Remove this (lines 39-52 in statefulset.yaml)
- name: permissions-fix
  securityContext:
    runAsUser: 0  # REMOVE: don't run as root

# Already configured correctly in podSecurityContext (lines 71-75)
podSecurityContext:
  fsGroup: 999
  runAsUser: 999
  runAsGroup: 999
  fsGroupChangePolicy: "OnRootMismatch"

# Add to securityContext (lines 78-86)
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
  readOnlyRootFilesystem: true  # NEW: add this
  runAsNonRoot: true
  runAsUser: 999
  seccompProfile:  # NEW: add seccomp
    type: RuntimeDefault
```

---

## User Interactions

### Docker Build Process
```bash
# Developers build image locally
docker build --target pgconfig-builder -t postgres-cli:latest .
docker build -t postgres:17-hardened .

# CI/CD pipeline
docker build --build-arg PG_VERSION=17 -t ghcr.io/flanksource/postgres:17 .
trivy image --severity HIGH,CRITICAL ghcr.io/flanksource/postgres:17
syft ghcr.io/flanksource/postgres:17 -o spdx-json > sbom.json
docker push ghcr.io/flanksource/postgres:17
```

### Helm Deployment
```bash
# Deploy with hardened configuration
helm install postgres ./chart \
  --set persistence.enabled=true \
  --set persistence.size=20Gi \
  --set persistence.retentionPolicy=Retain \
  --set conf.fsync=on \
  --set conf.synchronous_commit=on

# Validate security configuration
kubectl get pod postgres-0 -o yaml | grep -A 10 securityContext

# Check vulnerability scan status
kubectl get pod postgres-0 -o jsonpath='{.metadata.annotations}'
```

### Operational Workflows
1. **Before Upgrade**: Review SBOM and CVE scan results
2. **During Deployment**: Monitor init container logs for validation
3. **After Deployment**: Verify data integrity and PostgreSQL health
4. **Maintenance**: Review weekly vulnerability scans
5. **Incident Response**: Check SBOM for affected components

---

## Technical Considerations

### Current Architecture
- Multi-version PostgreSQL support (14-17) in single image
- Multi-stage build: Go builder + Debian final stage
- Custom postgres-cli for auto-upgrade functionality
- StatefulSet with single replica model
- Volume persistence with fsGroup-based permissions

### Integration Points
1. **GitHub Actions**: Build, scan, and push workflow
2. **Container Registry**: ghcr.io/flanksource/postgres
3. **Kubernetes**: StatefulSet with PVC
4. **PostgreSQL**: Versions 14, 15, 16, 17 official packages
5. **APT Repository**: apt.postgresql.org/pub/repos/apt

### Data Flow
```
Build Time:
Source Code → Go Build → postgres-cli binary
Dockerfile → Multi-stage Build → Optimized Image → Vulnerability Scan → SBOM Generation → Registry

Deployment Time:
Helm Chart → K8s API → StatefulSet
    ↓
Init Container: PVC Validation → Postgres Upgrade
    ↓
Main Container: PostgreSQL Start → Health Checks → Ready for Traffic

Runtime:
Application → Service → PostgreSQL Pod → PVC (persistent storage)
```

### Security Considerations
- **Least Privilege**: Run as postgres user (UID 999), no root access
- **Immutable Infrastructure**: Read-only root filesystem where possible
- **Defense in Depth**: Multiple security layers (image scan, runtime security, network policies)
- **Auditability**: SBOM and scan results for compliance tracking
- **Supply Chain**: Verified packages with GPG signatures

### Performance Considerations
- Image size reduction improves pull times and storage costs
- Combined RUN commands reduce layer overhead
- Proper fsync configuration may impact write performance (acceptable tradeoff for durability)
- Init container validation adds ~5-10s to startup time (acceptable for data safety)

### Constraints
- Must support PostgreSQL versions 14-17 in single image
- Must maintain compatibility with existing auto-upgrade functionality
- Must support multi-architecture (AMD64, ARM64)
- Single-replica deployment model (no multi-master or replication changes)

---

## Success Criteria

### Security
- [ ] Zero HIGH or CRITICAL CVEs in latest image scan
- [ ] All packages pinned with explicit versions
- [ ] SBOM generated and attached to every image build
- [ ] No root access required in any container (including init containers)
- [ ] Supply chain verified with GPG signatures

### Reliability
- [ ] Data integrity validated on startup with checksums
- [ ] PVC properly bound and writable before PostgreSQL starts
- [ ] Graceful shutdown with transaction completion guarantee
- [ ] No data loss during pod restarts or node failures
- [ ] Probes tuned to prevent false positives under load

### Operations
- [ ] Image size reduced by at least 15%
- [ ] Build time not increased by more than 20%
- [ ] Automated vulnerability scanning in CI/CD
- [ ] Clear documentation for version updates
- [ ] Runbooks for common security and persistence issues

### Validation Methods
1. **Vulnerability Scanning**: `trivy image ghcr.io/flanksource/postgres:17`
2. **SBOM Verification**: `syft ghcr.io/flanksource/postgres:17 -o json`
3. **Security Context Test**: `kubectl auth can-i --as=system:serviceaccount:default:postgres escalate`
4. **Data Integrity**: PostgreSQL checksum verification (`SELECT pg_stat_database.checksum_failures`)
5. **Image Size**: `docker images --format "{{.Size}}" ghcr.io/flanksource/postgres:17`
6. **Build Reproducibility**: Build same commit twice, compare image hashes

---

## Testing Requirements

### Unit Tests
- **Dockerfile**: Hadolint linting for Dockerfile best practices
- **Helm Chart**: `helm lint` and `ct lint` for chart validation
- **Version Pinning**: Script to verify all versions are pinned (no `latest` or unpinned packages)

### Integration Tests
- **Build Test**: Full image build on multiple architectures (AMD64, ARM64)
- **Security Scan Test**: Trivy scan passes with zero HIGH/CRITICAL CVEs
- **SBOM Generation**: Syft generates valid SPDX/CycloneDX SBOM
- **Helm Deployment**: Chart deploys successfully with persistence enabled
- **Init Container**: PVC validation logic tested with invalid PVC configuration
- **Data Persistence**: Data survives pod deletion and recreation
- **Upgrade Test**: Auto-upgrade from PG 14 → 17 preserves data integrity

### Security Tests
- **Privilege Escalation**: Attempt privilege escalation from container (should fail)
- **Root Filesystem**: Attempt to write to read-only filesystem (should fail)
- **Syscall Restriction**: Test seccomp profile blocks unauthorized syscalls
- **Network Policy**: Test network isolation if implemented
- **Secret Exposure**: Scan image layers for secrets or credentials

### Performance Tests
- **Image Pull Time**: Measure improvement from size reduction
- **Startup Time**: Measure init container overhead (target <30s)
- **Write Performance**: Benchmark fsync impact on write throughput
- **Failover Time**: Measure time to restart pod and restore service

---

## Implementation Checklist

### Phase 1: Analysis & Planning
- [ ] Review current CVE scan results for baseline
- [ ] Identify specific package versions to pin
- [ ] Determine SHA256 digests for base images
- [ ] Create test plan and validation scripts
- [ ] Set up vulnerability scanning in CI/CD
- [ ] Document rollback procedures

### Phase 2: Dockerfile Hardening
- [ ] Pin Go builder base image with SHA256
- [ ] Pin Debian base image with SHA256
- [ ] Pin all PostgreSQL package versions (14-17)
- [ ] Pin utility packages (gosu, jq, curl, etc.)
- [ ] Combine RUN commands to reduce layers
- [ ] Remove unnecessary packages from final image
- [ ] Add GPG signature verification for APT packages
- [ ] Create .dockerignore file
- [ ] Add multi-stage build cleanup
- [ ] Test build on AMD64 and ARM64

### Phase 3: Vulnerability Scanning Integration
- [ ] Add Trivy scanning to GitHub Actions workflow
- [ ] Configure HIGH/CRITICAL CVE build failure
- [ ] Add Syft SBOM generation step
- [ ] Attach SBOM to container image
- [ ] Set up GitHub Security alerts integration
- [ ] Create scheduled scan workflow (weekly)
- [ ] Document CVE remediation process

### Phase 4: Helm Chart Hardening
- [ ] Remove permissions-fix init container (use fsGroup)
- [ ] Add PVC validation to postgres-upgrade init container
- [ ] Add disk space check before PostgreSQL start
- [ ] Configure fsync and synchronous_commit in values
- [ ] Add WAL archiving configuration options
- [ ] Set PVC retention policy to Retain
- [ ] Add security scanning annotations
- [ ] Implement PreStop hook for graceful shutdown
- [ ] Tune probe timeouts and thresholds
- [ ] Add seccomp profile configuration

### Phase 5: Security Enhancements
- [ ] Enable read-only root filesystem with tmpfs mounts
- [ ] Create and test seccomp profile
- [ ] Document AppArmor/SELinux profiles
- [ ] Add PodSecurityStandard labels
- [ ] Validate RBAC permissions for service account
- [ ] Create network policy template (optional)
- [ ] Test privilege escalation prevention
- [ ] Verify capability dropping

### Phase 6: Testing & Validation
- [ ] Run Hadolint on Dockerfile
- [ ] Run helm lint and ct lint on chart
- [ ] Execute integration tests (build, deploy, persist)
- [ ] Run security tests (privilege escalation, filesystem)
- [ ] Perform upgrade testing (PG 14 → 17)
- [ ] Benchmark performance impact
- [ ] Test multi-architecture builds
- [ ] Validate SBOM generation

### Phase 7: Documentation & Rollout
- [ ] Update README with security hardening details
- [ ] Document version pinning update process
- [ ] Create runbook for CVE remediation
- [ ] Document data integrity validation procedures
- [ ] Create disaster recovery guide
- [ ] Write upgrade guide for existing deployments
- [ ] Prepare release notes
- [ ] Create security advisory (if addressing specific CVEs)

---

## Implementation Notes

### Critical Path Items
1. **Version Pinning** (FR-1) must be completed first - blocks vulnerability scanning
2. **Vulnerability Scanning** (FR-4) needed to validate all other improvements
3. **Persistent State** (FR-5) should be done early - impacts testing
4. **Image Optimization** (FR-2) can be done in parallel with security work

### Dependencies
- Trivy/Grype scanner availability in CI/CD environment
- Access to PostgreSQL APT repository for version lookups
- Kubernetes cluster for testing Helm deployments
- Multi-architecture build capability (buildx or similar)

### Risks & Mitigations
| Risk | Impact | Mitigation |
|------|--------|------------|
| Pinned version unavailable in APT repo | Build failure | Pin to stable versions with LTS support, test before pinning |
| Image size increase from SBOM | Slight increase | Use multi-stage builds, compress SBOM |
| fsync reduces write performance | Slower writes | Acceptable tradeoff for durability, benchmark first |
| Init container adds startup latency | Slower startup | Optimize validation logic, parallel checks |
| Read-only filesystem breaks PostgreSQL | Runtime failure | Test thoroughly, identify all writable directories |

### Version Update Process
1. Monitor security advisories for PostgreSQL, Debian, Go
2. Test new versions in staging environment
3. Update Dockerfile with new pinned versions and SHA256s
4. Run full test suite including security scans
5. Update documentation with new versions
6. Release with security advisory if addressing CVEs

---

## References

### Security Standards
- CIS Docker Benchmark: https://www.cisecurity.org/benchmark/docker
- CIS Kubernetes Benchmark: https://www.cisecurity.org/benchmark/kubernetes
- NIST Application Container Security Guide: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-190.pdf
- Pod Security Standards: https://kubernetes.io/docs/concepts/security/pod-security-standards/

### Tools
- Trivy: https://github.com/aquasecurity/trivy
- Grype: https://github.com/anchore/grype
- Syft: https://github.com/anchore/syft
- Hadolint: https://github.com/hadolint/hadolint

### PostgreSQL Security
- PostgreSQL Security Hardening: https://www.postgresql.org/docs/current/runtime-config-connection.html
- WAL Archiving: https://www.postgresql.org/docs/current/continuous-archiving.html
- Data Checksums: https://www.postgresql.org/docs/current/app-initdb.html#APP-INITDB-DATA-CHECKSUMS

---

**Document Version**: 1.0
**Created**: 2025-10-27
**Priority**: Critical
**Estimated Effort**: 3-5 days for full implementation
**Target Completion**: Immediate (production blocking)
