package swagger

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// openapi.yaml di sini adalah SALINAN HASIL GENERASI dari packages/contracts/openapi.yaml
// (sumber tunggal). Go embed tidak bisa membaca di luar paket, jadi salinan ini
// disinkronkan dengan `make openapi/sync`; TestEmbeddedSpecMatchesContracts gagal
// bila keduanya berbeda. Jangan sunting berkas ini langsung.
//
//go:embed openapi.yaml
var openapiFS embed.FS

// Spec mengembalikan spesifikasi OpenAPI yang disajikan di /swagger/openapi.yaml.
func Spec() ([]byte, error) { return openapiFS.ReadFile("openapi.yaml") }

// enabled menentukan apakah Swagger UI disajikan. Bawaan: aktif selain
// GIN_MODE=release. SWAGGER_ENABLED=true/false menimpa bawaan (mis. staging release).
func enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SWAGGER_ENABLED"))) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return os.Getenv("GIN_MODE") != "release"
}

// Register adds Swagger UI routes to the router. Disabled in release mode
// unless SWAGGER_ENABLED=true.
func Register(router *gin.Engine) {
	if !enabled() {
		return
	}

	// Redirect /swagger to /swagger/
	router.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/")
	})

	// Single wildcard route handles both the YAML file and the UI
	router.GET("/swagger/*any", func(c *gin.Context) {
		path := c.Param("any")

		// Serve the OpenAPI YAML file
		if path == "/openapi.yaml" {
			data, err := Spec()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "OpenAPI spec not found",
				})
				return
			}
			c.Header("Content-Type", "application/x-yaml")
			c.String(http.StatusOK, string(data))
			return
		}

		// Serve Swagger UI for all other paths under /swagger/
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host

		if fwdProto := c.GetHeader("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		}
		if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
			host = strings.Split(fwdHost, ",")[0]
		}
		specURL := scheme + "://" + host + "/swagger/openapi.yaml"

		tmpl := template.Must(template.New("swagger").Parse(swaggerHTML))
		c.Header("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(c.Writer, map[string]string{
			"SpecURL": specURL,
			"Title":   "Sistem Pemantauan Pangan dan Inflasi Aceh - API Documentation",
		})
	})
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>
    html { box-sizing: border-box; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "{{.SpecURL}}",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIBundle.SwaggerUIStandalonePreset
      ],
      plugins: [
        SwaggerUIBundle.plugins.DownloadUrl
      ],
      layout: "BaseLayout",
      docExpansion: "list",
      defaultModelsExpandDepth: 1,
      defaultModelExpandDepth: 1,
      filter: true,
      showExtensions: true,
      showCommonExtensions: true,
      tryItOutEnabled: true,
    });
  </script>
</body>
</html>`
