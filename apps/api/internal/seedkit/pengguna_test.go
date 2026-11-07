package seedkit

import (
	"testing"

	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
)

func TestIDPengguna_UUIDStabilDariSlug(t *testing.T) {
	id := IDPengguna("superadmin")

	assert.True(t, idx.ValidUUID(id))
	assert.Equal(t, idx.UUIDStabil("pengguna", "superadmin"), id, "rumus yang sama dengan migrasi 000110 (garda_uuid_stabil)")
	assert.Equal(t, id, IDPengguna("superadmin"), "deterministik")
	assert.NotEqual(t, id, IDPengguna("mahasiswa_1"))
}

func TestSlugPengguna_BalikanAkunSeedDanAkunBiasa(t *testing.T) {
	for _, slug := range SlugDemo {
		assert.Equal(t, slug, SlugPengguna(IDPengguna(slug)), slug)
	}

	acak := "6f1c1a52-0a57-4b9a-8f0e-0d1f6f3c9a11"
	assert.Equal(t, acak, SlugPengguna(acak), "akun buatan aplikasi dipakai apa adanya")
	assert.Equal(t, "bukan-uuid", SlugPengguna("bukan-uuid"))
}

func TestSlugDemo_TanpaDuplikat(t *testing.T) {
	terlihat := map[string]bool{}
	for _, slug := range SlugDemo {
		assert.False(t, terlihat[slug], "slug ganda: %s", slug)
		terlihat[slug] = true
	}
}
