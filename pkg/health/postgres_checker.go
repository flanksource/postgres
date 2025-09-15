package health

import "fmt"

// Postgres interface defines the methods needed for PostgreSQL health checking
type Postgres interface {
	SQL(query string) ([]map[string]interface{}, error)
}

// PostgreSQLChecker implements health check for PostgreSQL using the service's SQL method
type PostgreSQLChecker struct {
	postgres Postgres
}

// NewPostgreSQLChecker creates a new PostgreSQL health checker
func NewPostgreSQLChecker(postgres Postgres) *PostgreSQLChecker {
	return &PostgreSQLChecker{
		postgres: postgres,
	}
}

// Status performs a PostgreSQL health check
func (p *PostgreSQLChecker) Status() (interface{}, error) {
	if p.postgres == nil {
		return map[string]interface{}{
			"status": "unknown",
			"error":  "service not configured",
		}, fmt.Errorf("PostgreSQL service not configured")
	}

	_, err := p.postgres.SQL("SELECT 1")
	if err != nil {
		return "unhealthy", err
	}
	return "healthy", nil
}
