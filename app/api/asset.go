package api

import (
	"embed"
)

//go:embed static/*
var assetsFS embed.FS

//go:embed templates/*
var templatesFS embed.FS
