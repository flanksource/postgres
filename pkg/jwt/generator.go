package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/flanksource/postgres/pkg/utils"
)

// JWTGenerator handles JWT token creation and validation
type JWTGenerator struct {
	secret   utils.SensitiveString
	issuer   string
	audience string
}

// NewJWTGenerator creates a new JWT generator with the given secret and optional issuer/audience
func NewJWTGenerator(secret utils.SensitiveString, issuer, audience string) *JWTGenerator {
	return &JWTGenerator{
		secret:   secret,
		issuer:   issuer,
		audience: audience,
	}
}

// GenerateToken generates a JWT token with the specified role and expiry
func (j *JWTGenerator) GenerateToken(role string, expiry time.Duration, claims map[string]interface{}) (string, error) {
	if j.secret.IsEmpty() {
		return "", fmt.Errorf("JWT secret is not configured")
	}

	now := time.Now()

	// Build standard claims
	tokenClaims := jwt.MapClaims{
		"iat":  now.Unix(),
		"exp":  now.Add(expiry).Unix(),
		"role": role,
	}

	// Add optional issuer
	if j.issuer != "" {
		tokenClaims["iss"] = j.issuer
	}

	// Add optional audience
	if j.audience != "" {
		tokenClaims["aud"] = j.audience
	}

	// Add custom claims (they override standard claims if there's a conflict)
	for k, v := range claims {
		tokenClaims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	return token.SignedString([]byte(j.secret.Value()))
}

// GenerateHealthCheckToken generates a short-lived token for health check purposes
func (j *JWTGenerator) GenerateHealthCheckToken(adminRole string) (string, error) {
	claims := map[string]interface{}{
		"purpose": "health-check",
		"aud":     "postgrest-health-check",
	}

	// Short expiry for health checks (5 minutes)
	return j.GenerateToken(adminRole, 5*time.Minute, claims)
}

// GenerateServiceToken generates a long-lived service token
func (j *JWTGenerator) GenerateServiceToken(role string, service string, expiry time.Duration) (string, error) {
	claims := map[string]interface{}{
		"service": service,
		"type":    "service-token",
	}

	return j.GenerateToken(role, expiry, claims)
}

// ValidateToken parses and validates a JWT token
func (j *JWTGenerator) ValidateToken(tokenString string) (*jwt.Token, error) {
	if j.secret.IsEmpty() {
		return nil, fmt.Errorf("JWT secret is not configured")
	}

	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(j.secret.Value()), nil
	})
}

// GetClaims extracts and validates claims from a token
func (j *JWTGenerator) GetClaims(tokenString string) (jwt.MapClaims, error) {
	token, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// GetRole extracts the role from a token
func (j *JWTGenerator) GetRole(tokenString string) (string, error) {
	claims, err := j.GetClaims(tokenString)
	if err != nil {
		return "", err
	}

	role, ok := claims["role"].(string)
	if !ok {
		return "", fmt.Errorf("role claim not found or not a string")
	}

	return role, nil
}

// IsExpired checks if a token is expired
func (j *JWTGenerator) IsExpired(tokenString string) (bool, error) {
	claims, err := j.GetClaims(tokenString)
	if err != nil {
		return false, err
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return false, fmt.Errorf("exp claim not found or not a number")
	}

	return time.Unix(int64(exp), 0).Before(time.Now()), nil
}

// RemainingTime returns the remaining time until token expiry
func (j *JWTGenerator) RemainingTime(tokenString string) (time.Duration, error) {
	claims, err := j.GetClaims(tokenString)
	if err != nil {
		return 0, err
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return 0, fmt.Errorf("exp claim not found or not a number")
	}

	expiryTime := time.Unix(int64(exp), 0)
	remaining := expiryTime.Sub(time.Now())

	if remaining < 0 {
		return 0, nil // Already expired
	}

	return remaining, nil
}

// Config holds configuration for JWT generator
type Config struct {
	Secret   utils.SensitiveString `koanf:"secret" env:"JWT_SECRET" json:"secret"`
	Issuer   string                `koanf:"issuer" env:"JWT_ISSUER" json:"issuer" default:"postgres-config"`
	Audience string                `koanf:"audience" env:"JWT_AUDIENCE" json:"audience" default:"postgrest"`
}

// NewJWTGeneratorFromConfig creates a JWT generator from configuration
func NewJWTGeneratorFromConfig(config *Config) *JWTGenerator {
	return NewJWTGenerator(config.Secret, config.Issuer, config.Audience)
}
