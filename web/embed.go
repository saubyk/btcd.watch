// Package web embeds the built SPA so the production server ships as a
// single binary. dist/.keep is committed to keep the embed valid before
// the first frontend build; `make build` populates dist/ for real.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
