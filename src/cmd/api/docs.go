//go:generate swag init -g docs.go -o ../../docs --parseDependency --parseInternal --dir .,../../internal/httpapi

package main

// @title sniply_api API
// @version 1.0
// @description sniply_api HTTP API.
// @BasePath /v1
// @securityDefinitions.apikey SessionAuth
// @in cookie
// @name sniply_session
// @description HttpOnly session cookie
