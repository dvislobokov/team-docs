//go:build prod

package main

import (
	"io/fs"

	"github.com/dvislobokov/srog"

	"team-docs/internal/server"
	"team-docs/web"
)

// registerStatic в prod-сборке отдаёт встроенный фронт из web/dist.
func registerStatic(srv *server.Server, log *srog.Logger) {
	dist, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatal(err, "failed to sub embedded dist fs")
	}
	srv.RegisterStatic(dist)
	log.Information("prod build: serving embedded frontend from web/dist")
}
