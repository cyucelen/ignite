package ui

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

//go:embed build/*
var embedFS embed.FS

func SetupRoutes(app *fiber.App) error {
	buildFS, err := fs.Sub(embedFS, "build")
	if err != nil {
		return err
	}

	app.Use("/assets", filesystem.New(filesystem.Config{
		Root:   http.FS(buildFS),
		Browse: false,
	}))

	app.Use("/", filesystem.New(filesystem.Config{
		Root:   http.FS(buildFS),
		Browse: false,
		Next: func(c *fiber.Ctx) bool {
			path := c.Path()
			return strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/ws")
		},
	}))

	app.Use(func(c *fiber.Ctx) error {
		if !strings.HasPrefix(c.Path(), "/api") && !strings.HasPrefix(c.Path(), "/ws") {
			return c.SendFile("./internal/ui/build/index.html")
		}
		return c.Next()
	})

	log.Println("UI routes configured: serving embedded React app from /")
	return nil
}

func HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"service": "ui",
	})
}
