package health

import "fmt"

// PgBouncer interface defines the methods needed for PgBouncer health checking
type PgBouncer interface {
	Health() error
}

// PgBouncerChecker implements health check for PgBouncer using the service's Health method
type PgBouncerChecker struct {
	pgbouncer PgBouncer
}

// NewPgBouncerChecker creates a new PgBouncer health checker
func NewPgBouncerChecker(pgbouncer PgBouncer) *PgBouncerChecker {
	return &PgBouncerChecker{
		pgbouncer: pgbouncer,
	}
}

// Status performs a PgBouncer health check
func (p *PgBouncerChecker) Status() (interface{}, error) {
	if p.pgbouncer == nil {
		return map[string]interface{}{
			"status": "unknown",
			"error":  "service not configured",
		}, fmt.Errorf("PgBouncer service not configured")
	}

	err := p.pgbouncer.Health()
	if err != nil {
		return "unhealthy", err
	}
	return "healthy", nil
}
