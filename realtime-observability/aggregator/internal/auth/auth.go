package auth

import (
	"context"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Authenticator handles API key authentication
type Authenticator struct {
	apiKeys map[string]bool
	enabled bool
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator() *Authenticator {
	auth := &Authenticator{
		apiKeys: make(map[string]bool),
		enabled: false,
	}

	// Load API keys from environment
	keysEnv := os.Getenv("TELEMETRY_API_KEYS")
	if keysEnv != "" {
		auth.enabled = true
		keys := strings.Split(keysEnv, ",")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				auth.apiKeys[key] = true
			}
		}
		log.Printf("Authentication enabled with %d API keys", len(auth.apiKeys))
	} else {
		log.Println("Authentication disabled (no TELEMETRY_API_KEYS set)")
	}

	return auth
}

// ValidateAPIKey checks if the provided API key is valid
func (a *Authenticator) ValidateAPIKey(key string) bool {
	if !a.enabled {
		return true
	}
	return a.apiKeys[key]
}

// UnaryInterceptor returns a gRPC unary interceptor for authentication
func (a *Authenticator) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := a.authenticate(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream interceptor for authentication
func (a *Authenticator) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := a.authenticate(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// authenticate validates the API key from context metadata
func (a *Authenticator) authenticate(ctx context.Context) error {
	if !a.enabled {
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	keys := md.Get("x-api-key")
	if len(keys) == 0 {
		return status.Error(codes.Unauthenticated, "missing API key")
	}

	if !a.ValidateAPIKey(keys[0]) {
		return status.Error(codes.PermissionDenied, "invalid API key")
	}

	return nil
}

// AddAPIKey adds a new API key at runtime
func (a *Authenticator) AddAPIKey(key string) {
	a.apiKeys[key] = true
	a.enabled = true
}

// RemoveAPIKey removes an API key
func (a *Authenticator) RemoveAPIKey(key string) {
	delete(a.apiKeys, key)
}

// Enable enables authentication
func (a *Authenticator) Enable() {
	a.enabled = true
}

// Disable disables authentication
func (a *Authenticator) Disable() {
	a.enabled = false
}
