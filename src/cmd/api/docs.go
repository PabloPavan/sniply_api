//go:generate swag init -g docs.go -o ../../docs --parseDependency --parseInternal --dir .,../../internal/httpapi

package main

// @title sniply_api API
// @version 1.0
// @description sniply_api HTTP API.
// @BasePath /v1
// @securityDefinitions.apikey SessionAuth
// @in header
// @name Cookie
// @description Cookie header with sniply_session
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key (X-API-Key or Authorization: Bearer)
