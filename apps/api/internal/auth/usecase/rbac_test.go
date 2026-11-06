package usecase

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	permissionsContractPath = "../../../../../packages/contracts/permissions.json"
	architecturePath        = "../../../../../docs/ARCHITECTURE.md"
	internalDir             = "../.."
)

var updatePermissions = flag.Bool("update-permissions", false, "tulis ulang packages/contracts/permissions.json dari rolePermissions")

// permissionsContract adalah bentuk packages/contracts/permissions.json, yang
// juga dibaca tes web agar keduanya tidak menyimpang dari peta di kode.
type permissionsContract struct {
	Permissions []string            `json:"permissions"`
	Roles       map[string][]string `json:"roles"`
}

func currentContract() permissionsContract {
	contract := permissionsContract{Permissions: []string{}, Roles: map[string][]string{}}
	for role, perms := range rolePermissions {
		contract.Roles[role] = slices.Sorted(slices.Values(perms))
	}
	// Katalog izin didefinisikan di kode (rbac_katalog.go); matriks di berkas ini hanyalah bawaannya.
	for _, d := range katalogIzin {
		contract.Permissions = append(contract.Permissions, d.Nama)
	}
	sort.Strings(contract.Permissions)
	return contract
}

func TestKatalogIzin_LengkapUnikDanKonsistenDenganMatriksBawaan(t *testing.T) {
	assert.Len(t, katalogIzin, 35)
	nama := regexp.MustCompile(`^[a-z]+\.[a-z_]+$`)
	seen := map[string]bool{}
	for _, d := range katalogIzin {
		assert.Regexp(t, nama, d.Nama)
		assert.False(t, seen[d.Nama], "duplikat di katalog: %s", d.Nama)
		assert.NotEmpty(t, d.Deskripsi, d.Nama)
		assert.NotEmpty(t, d.Kelompok, d.Nama)
		seen[d.Nama] = true
	}

	dimiliki := map[string]bool{}
	for role, perms := range rolePermissions {
		for _, p := range perms {
			assert.True(t, seen[p], "izin bawaan %s milik %s tidak ada di katalog", p, role)
			dimiliki[p] = true
		}
	}
	for p := range seen {
		assert.True(t, dimiliki[p], "izin katalog %s tidak dimiliki peran manapun secara bawaan", p)
	}
}

func TestDaftarPeran_EmpatBelasPeranBerurutanDenganLabelDanLapis(t *testing.T) {
	peran := DaftarPeran()
	require.Len(t, peran, 14)
	for i, p := range peran {
		assert.Equal(t, roleOrder[i], p.Kunci)
		assert.Equal(t, i+1, p.Urutan)
		assert.NotEmpty(t, p.Label, p.Kunci)
		assert.Equal(t, roleLapis[p.Kunci], p.Lapis)
		assert.True(t, PeranDikenal(p.Kunci))
	}
	assert.False(t, PeranDikenal("SUPERADMIN"))
}

func TestIzinBawaan_SalinanDanPeranTakDikenalNil(t *testing.T) {
	bawaan := IzinBawaan("ADMIN_SISTEM")
	require.NotEmpty(t, bawaan)
	bawaan[0] = "diubah.tes"
	assert.NotEqual(t, "diubah.tes", rolePermissions["ADMIN_SISTEM"][0], "pemanggil tidak boleh mengubah matriks bawaan")
	assert.Nil(t, IzinBawaan("SUPERADMIN"))

	katalog := KatalogIzin()
	katalog[0].Nama = "diubah.tes"
	assert.NotEqual(t, "diubah.tes", katalogIzin[0].Nama)
	assert.True(t, IzinDikenal("pengguna.kelola"))
	assert.False(t, IzinDikenal("izin.palsu"))
}

func TestPermissionsContractFileMatchesRolePermissions(t *testing.T) {
	want := currentContract()

	if *updatePermissions {
		raw, err := json.MarshalIndent(want, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(permissionsContractPath, append(raw, '\n'), 0o644))
	}

	raw, err := os.ReadFile(permissionsContractPath)
	require.NoError(t, err)
	var got permissionsContract
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, want, got, "permissions.json basi; jalankan: go test ./internal/auth/usecase -run TestPermissionsContractFile -update-permissions")
}

func TestEveryRoleInOrderHasPermissionsAndLayer(t *testing.T) {
	assert.Len(t, roleOrder, 14)
	for _, role := range roleOrder {
		assert.NotEmpty(t, rolePermissions[role], "peran %s tanpa izin", role)
		assert.NotEmpty(t, roleLapis[role], "peran %s tanpa lapis", role)
	}
	assert.Len(t, rolePermissions, len(roleOrder), "ada peran di peta izin yang tidak ada di roleOrder")
}

var (
	permissionCall    = regexp.MustCompile(`(?:RequirePermission|RequireAnyPermission|HasPermission)\(([^)]*)\)`)
	permissionLiteral = regexp.MustCompile(`"([a-z_]+\.[a-z_]+)"`)
)

// Setiap izin yang diminta route harus ada di daftar; izin yatim membuat route
// selalu 403 (temuan 02-A: lapangan.lihat, prediksi.baca).
func TestEveryPermissionRequiredByRoutesIsKnown(t *testing.T) {
	known := map[string]bool{}
	for _, p := range currentContract().Permissions {
		known[p] = true
	}

	used := map[string][]string{}
	require.NoError(t, filepath.WalkDir(internalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, call := range permissionCall.FindAllStringSubmatch(string(src), -1) {
			for _, lit := range permissionLiteral.FindAllStringSubmatch(call[1], -1) {
				used[lit[1]] = append(used[lit[1]], path)
			}
		}
		return nil
	}))

	require.NotEmpty(t, used, "tidak ada pemanggilan RequirePermission yang ditemukan; pola pencarian rusak")
	for perm, files := range used {
		assert.True(t, known[perm], "izin %q diminta di %v tetapi tidak dimiliki peran manapun", perm, files)
	}
}

// Peta izin harus sama dengan matriks rute x peran di docs/ARCHITECTURE.md.
func TestRolePermissionsMatchArchitectureMatrix(t *testing.T) {
	raw, err := os.ReadFile(architecturePath)
	require.NoError(t, err)

	rows := 0
	for _, line := range strings.Split(string(raw), "\n") {
		route, perm, cells, ok := parseMatrixRow(line)
		if !ok {
			continue
		}
		rows++
		require.Len(t, cells, len(roleOrder), "baris %s", route)
		for i, role := range roleOrder {
			want := cells[i] == "✓"
			assert.Equal(t, want, slices.Contains(rolePermissions[role], perm), "%s (%s) untuk peran %s", route, perm, role)
		}
	}
	assert.GreaterOrEqual(t, rows, 15, "baris matriks tidak terbaca")
}

// parseMatrixRow membaca baris "| `/rute` | `izin` | ✓ | · ... |"; baris tanpa izin (—) dilewati.
func parseMatrixRow(line string) (route, perm string, cells []string, ok bool) {
	if !strings.HasPrefix(line, "| `/") {
		return "", "", nil, false
	}
	cols := strings.Split(strings.Trim(line, "| "), "|")
	if len(cols) < 3 {
		return "", "", nil, false
	}
	route, perm = strings.Trim(strings.TrimSpace(cols[0]), "`"), strings.Trim(strings.TrimSpace(cols[1]), "`")
	if !strings.Contains(perm, ".") {
		return "", "", nil, false
	}
	for _, c := range cols[2:] {
		cells = append(cells, strings.TrimSpace(c))
	}
	return route, perm, cells, true
}
