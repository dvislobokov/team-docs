//go:build !prod

package main

import (
	"github.com/dvislobokov/srog"

	"team-docs/internal/server"
)

// registerStatic в dev-сборке ничего не делает: фронт обслуживает Vite (:5173)
// с proxy на /api. Prod-версия (build tag prod) встраивает web/dist.
func registerStatic(_ *server.Server, log *srog.Logger) {
	log.Information("dev build: static frontend served by Vite, backend serves /api only")
}
