// Package auth provides some common functionality for implementing
// authentication and authorization in gRPC services.
package auth

// MetadataKey is the canonical key in gRPC metadata where
// authentication/authorization data is stored.
const MetadataKey = "authorization"
