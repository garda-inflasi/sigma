package swagger

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openapiFS embed.FS

// Register adds Swagger UI routes to the router in development mode.
// In production mode (GIN_MODE=release), Swagger UI is disabled.
func Register(router *gin.Engine) {
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "release" {
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
			data, err := openapiFS.ReadFile("openapi.yaml")
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
