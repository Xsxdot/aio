package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRegisterStaticFiles_DoesNotInterceptAdminRoutes(t *testing.T) {
	f := fiber.New()

	// Prepare a temp static dir (avoid sendfile not found if fallback ever triggers).
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	RegisterStaticFiles(f, staticDir, "/")

	// Register a normal admin route AFTER static catch-all.
	f.Get("/admin/admins/info", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/admins/info", nil)
	resp, err := f.Test(req)
	if err != nil {
		t.Fatalf("fiber test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d, body=%s", http.StatusOK, resp.StatusCode, string(body))
	}
}

