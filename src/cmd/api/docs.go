//go:generate swag init -g docs.go -o ../../docs --parseDependency --parseInternal --dir .,../../internal/httpapi

package main

// @title Sniply API
// @version 1.0
// @description Sniply HTTP API.
// @BasePath /v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer {token}" to authorize
