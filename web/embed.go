//go:build prod

// Package web встраивает собранный фронтенд (web/dist) в бинарь.
// Файл компилируется только с build-тегом prod; в dev-сборке его нет,
// а статику обслуживает Vite.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
