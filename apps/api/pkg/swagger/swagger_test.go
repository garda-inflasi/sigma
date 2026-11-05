package swagger

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

const contractPath = "../../../../packages/contracts/openapi.yaml"

func readContract(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("baca sumber tunggal %s: %v", contractPath, err)
	}
	return data
}

// Salinan yang di-embed harus identik dengan sumber tunggal.
// Perbaiki dengan: make openapi/sync
func TestEmbeddedSpecMatchesContracts(t *testing.T) {
	embedded, err := Spec()
	if err != nil {
		t.Fatalf("baca spesifikasi embed: %v", err)
	}
	if !bytes.Equal(embedded, readContract(t)) {
		t.Fatal("apps/api/pkg/swagger/openapi.yaml berbeda dari packages/contracts/openapi.yaml; jalankan `make openapi/sync`")
	}
}

func TestSpecIsValidOpenAPIWithoutLegacyAnalytics(t *testing.T) {
	embedded, err := Spec()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		OpenAPI string                    `yaml:"openapi"`
		Info    struct{ Title string }    `yaml:"info"`
		Paths   map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(embedded, &doc); err != nil {
		t.Fatalf("YAML tidak valid: %v", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Errorf("versi openapi %q bukan 3.x", doc.OpenAPI)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("spesifikasi tidak memuat path")
	}
	for _, must := range []string{"/healthz", "/v1/auth/login"} {
		if _, ok := doc.Paths[must]; !ok {
			t.Errorf("path %s tidak ada", must)
		}
	}
	for p := range doc.Paths {
		if strings.Contains(p, "analytics") {
			t.Errorf("path analytics lama masih ada: %s", p)
		}
	}
	if strings.Contains(doc.Info.Title, "Sistem Pemantauan Pangan") {
		t.Errorf("judul spesifikasi lama terdeteksi: %q", doc.Info.Title)
	}
}

func serve(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSwaggerServesCurrentSpec(t *testing.T) {
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("SWAGGER_ENABLED", "")
	rec := serve(t, "/swagger/openapi.yaml")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), readContract(t)) {
		t.Fatal("/swagger/openapi.yaml tidak sama dengan packages/contracts/openapi.yaml")
	}
	if ui := serve(t, "/swagger/"); ui.Code != http.StatusOK || !strings.Contains(ui.Body.String(), "swagger-ui") {
		t.Fatalf("UI tidak disajikan: %d", ui.Code)
	}
}

func TestSwaggerDisabledInReleaseUnlessForced(t *testing.T) {
	t.Setenv("GIN_MODE", "release")
	t.Setenv("SWAGGER_ENABLED", "")
	if rec := serve(t, "/swagger/openapi.yaml"); rec.Code != http.StatusNotFound {
		t.Fatalf("release harus menonaktifkan swagger, status %d", rec.Code)
	}
	t.Setenv("SWAGGER_ENABLED", "true")
	if rec := serve(t, "/swagger/openapi.yaml"); rec.Code != http.StatusOK {
		t.Fatalf("SWAGGER_ENABLED=true harus mengaktifkan, status %d", rec.Code)
	}
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("SWAGGER_ENABLED", "false")
	if rec := serve(t, "/swagger/openapi.yaml"); rec.Code != http.StatusNotFound {
		t.Fatalf("SWAGGER_ENABLED=false harus menonaktifkan, status %d", rec.Code)
	}
}
