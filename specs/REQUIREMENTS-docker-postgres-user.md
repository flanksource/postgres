# Feature: Docker Container Must Run as Postgres User (Not Root)

## Overview

This is a security fix to ensure the PostgreSQL Docker container runs as the postgres user (UID 999) instead of root by default. The current implementation runs as root which violates container security best practices and creates unnecessary security risks.

**Problem Being Solved**: The container currently runs as root (commented out `USER postgres` in Dockerfile line 121), leading to:
- Security vulnerabilities (running database as root)
- Permission issues with volume mounts
- Violation of least-privilege principle
- Potential data ownership problems

**Target Users**:
- DevOps engineers deploying PostgreSQL containers
- Security-conscious organizations
- Kubernetes administrators using this image

**Scope**: Breaking change acceptable - this is a security fix that requires migration for existing deployments.

## Functional Requirements

### FR-1: Dockerfile Must Define Postgres User
**Description**: The Dockerfile must uncomment and set `USER postgres` to ensure the container runs as the postgres user by default.

**User Story**: As a security engineer, I want the PostgreSQL container to run as a non-root user by default, so that the attack surface is minimized.

**Acceptance Criteria**:
- [ ] Dockerfile line 121 `USER postgres` is uncommented
- [ ] Container starts as UID 999 (postgres user) by default
- [ ] No root processes are running during normal operation
- [ ] `docker inspect` shows User: "postgres" in container config

### FR-2: Entrypoint Script Must Work as Postgres User
**Description**: The docker-entrypoint.sh script must operate correctly when running as the postgres user, without requiring root privileges for normal operations.

**User Story**: As a container operator, I want the entrypoint script to initialize and start PostgreSQL as the postgres user, so that no privilege escalation is needed.

**Acceptance Criteria**:
- [ ] Script runs successfully as postgres user (UID 999)
- [ ] Can read and write to PGDATA when owned by postgres
- [ ] postgres-cli auto-start executes with postgres user permissions
- [ ] PostgreSQL server starts successfully under postgres user
- [ ] No "permission denied" errors during normal startup

### FR-3: Permission Pre-flight Checks in postgres-cli
**Description**: The postgres-cli auto-start command must include pre-flight permission checks before attempting initialization or operations.

**User Story**: As a developer, I want clear error messages when PGDATA has incorrect permissions, so that I can quickly diagnose and fix permission issues.

**Acceptance Criteria**:
- [ ] Check PGDATA directory exists and is readable/writable before operations
- [ ] Verify current user has ownership or appropriate permissions
- [ ] Detect permission issues before attempting initdb or pg_upgrade
- [ ] Exit with clear error code (e.g., exit 126 for permission denied)

### FR-4: Informative Error Messages for Permission Failures
**Description**: When permission issues are detected, provide clear, actionable error messages that guide users to fix the problem.

**User Story**: As a user encountering permission errors, I want helpful error messages with exact commands to fix the issue, so that I don't waste time debugging.

**Acceptance Criteria**:
- [ ] Error message identifies the specific path with wrong permissions
- [ ] Shows current ownership (UID:GID) vs expected (999:999)
- [ ] Provides exact docker/kubectl command to fix permissions
- [ ] Explains difference between named volumes and bind mounts
- [ ] Example: "PGDATA /var/lib/postgresql/data is owned by root:root, expected postgres:postgres (999:999). Fix with: docker run --rm -v pgdata:/var/lib/postgresql/data --user root alpine chown -R 999:999 /var/lib/postgresql/data"

### FR-5: Dry-run Mode for Permission Validation
**Description**: Add `--dry-run` flag to postgres-cli auto-start that validates permissions without making changes.

**User Story**: As a DevOps engineer, I want to validate that volume permissions are correct before starting the database, so that I can catch issues in CI/CD pipelines.

**Acceptance Criteria**:
- [ ] `postgres-cli auto-start --dry-run` validates all permission requirements
- [ ] Checks PGDATA read/write access
- [ ] Checks config directory permissions
- [ ] Returns exit code 0 if all checks pass, non-zero otherwise
- [ ] Outputs validation results in structured format (can be JSON with flag)

### FR-6: Optional Root Mode for Permission Fixing
**Description**: Support running the container explicitly as root to fix permission issues, with clear warnings that this is not the default mode.

**User Story**: As a system administrator, I need a way to fix permission issues on existing volumes, so that I can migrate to the secure default.

**Acceptance Criteria**:
- [ ] Container can be started with `--user root` to run as root explicitly
- [ ] When running as root, entrypoint detects it and logs warning
- [ ] If PGDATA ownership is wrong and running as root, fix with chown and warn
- [ ] Warning message: "Running as root is not recommended. This will fix permissions and exit. Restart without --user root for normal operation."
- [ ] After fixing permissions, suggest restarting as postgres user

### FR-7: PGDATA Initialization as Postgres User
**Description**: Ensure PGDATA initialization (initdb) works correctly when running as postgres user.

**User Story**: As a new user, I want the container to initialize an empty database automatically when I start it for the first time, without needing root access.

**Acceptance Criteria**:
- [ ] Empty PGDATA directory (owned by postgres) initializes successfully
- [ ] initdb command runs as postgres user
- [ ] Default database and user are created correctly
- [ ] File permissions in PGDATA are correctly set (0700 for directories, 0600 for files)
- [ ] No root intervention required for fresh initialization

### FR-8: Migration Documentation
**Description**: Provide comprehensive documentation for migrating existing deployments from root to postgres user.

**User Story**: As an existing user, I need clear migration instructions, so that I can update my deployments without data loss or extended downtime.

**Acceptance Criteria**:
- [ ] Document breaking change in README.md with clear version information
- [ ] Provide step-by-step migration guide for Docker Compose
- [ ] Provide step-by-step migration guide for Kubernetes
- [ ] Include commands for checking and fixing volume permissions
- [ ] Document differences between named volumes and bind mounts
- [ ] Provide rollback procedure if issues occur

## User Interactions

### Docker CLI Interaction
**Normal startup (new default)**:
```bash
docker run -v pgdata:/var/lib/postgresql/data flanksource/postgres:latest
# Runs as postgres user, initializes if needed
```

**Permission fix mode (explicit root)**:
```bash
docker run --user root -v pgdata:/var/lib/postgresql/data flanksource/postgres:latest
# Detects wrong permissions, fixes them, logs warning
# User then restarts without --user root
```

**Dry-run validation**:
```bash
docker run -v pgdata:/var/lib/postgresql/data flanksource/postgres:latest --dry-run
# Validates permissions, exits with status code
```

### Kubernetes Interaction
```yaml
apiVersion: v1
kind: Pod
spec:
  securityContext:
    runAsUser: 999
    runAsGroup: 999
    fsGroup: 999
  containers:
  - name: postgres
    image: flanksource/postgres:latest
    volumeMounts:
    - name: pgdata
      mountPath: /var/lib/postgresql/data
```

### Error Handling Flow
1. Container starts as postgres user (UID 999)
2. Entrypoint runs postgres-cli auto-start
3. Pre-flight checks detect permission issue
4. Clear error message displayed with fix commands
5. Container exits with code 126
6. User fixes permissions or runs in root mode
7. Container restarts successfully

## Technical Considerations

### Container Runtime
- **Image**: Runs as postgres user (UID 999, GID 999) by default
- **Entrypoint**: Must not assume root privileges
- **Volume ownership**: PGDATA must be owned by postgres user
- **Security**: No privilege escalation required for normal operations

### Permission Model
- **Default**: Container runs as postgres (UID 999)
- **PGDATA**: Must be owned by 999:999 (postgres:postgres)
- **Config dir**: Must be writable by postgres user
- **Named volumes**: Docker/Kubernetes handles ownership automatically (usually correct)
- **Bind mounts**: Require manual ownership setup or init container

### Data Flow
1. **Startup**: docker-entrypoint.sh → postgres-cli auto-start → permission checks
2. **Initialization**: Check PGDATA → Run initdb as postgres → Configure → Start server
3. **Upgrade**: Check versions → Run pg_upgrade as postgres → Start new version
4. **Root mode**: Detect root user → Fix permissions → Warn → Continue or exit

### Integration Points
- **Dockerfile**: Define USER postgres
- **docker-entrypoint.sh**: Remove permission workarounds, add user detection
- **postgres-cli**: Add permission checks, dry-run mode, informative errors
- **Documentation**: Migration guides, examples, troubleshooting

### Security Requirements
- **Principle of least privilege**: Run as non-root by default
- **No setuid binaries**: Do not use setuid for privilege escalation
- **Clear audit trail**: Log user context at startup
- **Explicit root mode**: Require explicit --user root flag, not default

### Performance Considerations
- Permission checks add minimal overhead (< 100ms)
- Pre-flight checks prevent wasted initialization attempts
- Dry-run mode enables fast validation in CI/CD

## Success Criteria

**Overall definition of done**:
- [ ] Container runs as postgres user (UID 999) by default without `USER postgres` being commented
- [ ] Fresh PGDATA initialization works as postgres user
- [ ] Permission issues produce clear, actionable error messages
- [ ] Dry-run mode enables permission validation
- [ ] Root mode available for explicit permission fixing
- [ ] Migration documentation covers Docker and Kubernetes
- [ ] Tests validate postgres user operation with fresh PGDATA
- [ ] No regression in functionality when running as postgres user
- [ ] Security scan passes with no high-severity issues related to user privileges

## Testing Requirements

### Unit Tests
- postgres-cli permission check functions
- Error message formatting
- User detection logic
- Dry-run flag handling

### Integration Tests
- **Test 1: Fresh initialization as postgres user**
  - Start container with empty PGDATA volume
  - Verify initialization succeeds
  - Verify database is accessible
  - Verify all files owned by postgres:postgres

- **Test 2: Permission error detection**
  - Create PGDATA owned by root
  - Start container as postgres user
  - Verify clear error message
  - Verify exit code 126

- **Test 3: Root mode permission fix**
  - Create PGDATA owned by root
  - Start container with --user root
  - Verify ownership is fixed
  - Verify warning is logged
  - Restart as postgres user
  - Verify successful startup

- **Test 4: Dry-run validation**
  - Run with --dry-run on correct permissions
  - Verify exit code 0
  - Run with --dry-run on wrong permissions
  - Verify exit code non-zero and clear output

- **Test 5: Auto-upgrade as postgres user**
  - Start with PG 14 data
  - Upgrade to PG 17
  - Verify upgrade succeeds as postgres user
  - Verify data integrity

## Implementation Checklist

### Phase 1: Setup & Analysis
- [ ] Review current Dockerfile and identify all root-dependent operations
- [ ] Analyze docker-entrypoint.sh for permission assumptions
- [ ] Identify postgres-cli functions that need permission checks
- [ ] Document current permission issues and their causes

### Phase 2: Core Implementation

#### Dockerfile Changes
- [ ] Uncomment `USER postgres` at line 121
- [ ] Verify all directories created before USER directive are owned by postgres
- [ ] Ensure PGDATA, PGCONFIG_CONFIG_DIR owned by postgres (line 99)
- [ ] Test image build succeeds

#### docker-entrypoint.sh Changes
- [ ] Remove or update permission workaround comments (line 5)
- [ ] Add user detection: check if running as root vs postgres
- [ ] If root: warn, fix PGDATA ownership if needed, suggest restart
- [ ] If postgres: continue normal startup
- [ ] Remove any chown or chmod commands assuming root access

#### postgres-cli Changes
- [ ] Add pre-flight permission checks to auto-start command
- [ ] Implement `--dry-run` flag for permission validation
- [ ] Add clear error messages for permission failures
- [ ] Include fix commands in error output (docker/kubectl examples)
- [ ] Add user context logging (UID/GID at startup)
- [ ] Ensure initdb, pg_upgrade work correctly as postgres user

### Phase 3: Testing
- [ ] Write unit tests for permission check functions
- [ ] Write integration test: fresh PGDATA as postgres user
- [ ] Write integration test: permission error detection and messages
- [ ] Write integration test: root mode permission fixing
- [ ] Write integration test: dry-run mode validation
- [ ] Test with Docker named volumes
- [ ] Test with Docker Compose
- [ ] Test with Kubernetes StatefulSet and PVC
- [ ] Verify no regression in auto-upgrade functionality

### Phase 4: Documentation & Migration Guide
- [ ] Update README.md with breaking change notice
- [ ] Write migration guide for Docker users
- [ ] Write migration guide for Kubernetes users
- [ ] Document bind mount setup requirements
- [ ] Add troubleshooting section for permission errors
- [ ] Include examples of permission fix commands
- [ ] Update health check documentation
- [ ] Add security best practices section

### Phase 5: Validation & Release
- [ ] Run `make lint` and fix any issues
- [ ] Run `make build` and verify success
- [ ] Run all integration tests
- [ ] Security scan with no high-severity privilege issues
- [ ] Update CHANGELOG.md with breaking change
- [ ] Tag release with appropriate version bump (major due to breaking change)
- [ ] Update container registry with new image

## Migration Guide Summary

### For Docker Users
```bash
# Check current volume ownership
docker run --rm -v pgdata:/data alpine ls -la /data

# If owned by root, fix permissions
docker run --rm -v pgdata:/data alpine chown -R 999:999 /data

# Then start normally (will run as postgres user)
docker run -v pgdata:/var/lib/postgresql/data flanksource/postgres:latest
```

### For Kubernetes Users
```yaml
# Add securityContext to pod spec
spec:
  securityContext:
    runAsUser: 999
    runAsGroup: 999
    fsGroup: 999  # Ensures PVC is owned by postgres
  containers:
  - name: postgres
    image: flanksource/postgres:latest
```

### Rollback Procedure
If issues occur, rollback to previous image version:
```bash
docker run flanksource/postgres:v1.x.x  # Previous version
```

## Notes

- This change aligns with Docker and Kubernetes security best practices
- Official PostgreSQL Docker images also run as postgres user
- Breaking change is justified by security benefits
- Clear migration path minimizes disruption
- Root mode provides escape hatch for edge cases
