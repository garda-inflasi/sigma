package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Riwayat peran dibaca dari audit, jadi setiap jalur yang mengubah peran harus mencatat peran sebelum dan
// sesudah, dan tidak boleh kehilangan catatan itu hanya karena langkah sesudahnya (pencabutan sesi) gagal.

func TestAssignRole_AuditTetapDitulisBilaPencabutanSesiGagal(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(errors.New("db putus"))

	_, err := uc.AssignRole(context.Background(), "user-123", "MANAJER_DATA", "admin-1")

	require.Error(t, err, "pencabutan sesi yang gagal tetap dilaporkan")
	entries := recorder.Entries()
	require.Len(t, entries, 1, "peran sudah berubah di basis data: harus ada jejaknya")
	assert.Equal(t, "ASSIGN_ROLE", entries[0].Aksi)
	assert.Equal(t, true, entries[0].Metadata["pencabutan_sesi_gagal"])
	assert.Equal(t, "OPS_MAHASISWA", entries[0].Sebelum.(map[string]any)["peran"])
	assert.Equal(t, "MANAJER_DATA", entries[0].Sesudah.(map[string]any)["peran"])
}

func TestUpdateUser_GantiPeranAuditTetapDitulisBilaPencabutanSesiGagal(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(errors.New("db putus"))
	peran := "OPS_KOORDINATOR"

	_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Peran: &peran}, "admin-1")

	require.Error(t, err)
	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "UPDATE_USER", entries[0].Aksi)
	assert.Equal(t, true, entries[0].Metadata["pencabutan_sesi_gagal"])
	assert.Equal(t, "OPS_MAHASISWA", entries[0].Sebelum.(map[string]any)["peran"])
	assert.Equal(t, "OPS_KOORDINATOR", entries[0].Sesudah.(map[string]any)["peran"])
}

// Setiap jalur yang mengubah peran menyertakan peran di snapshot sebelum DAN sesudah (CREATE_USER hanya sesudah).
func TestJalurPerubahanPeran_SnapshotMemuatPeran(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	peran := "MANAJER_DATA"

	_, err := uc.AssignRole(context.Background(), "user-123", peran, "admin-1")
	require.NoError(t, err)
	_, err = uc.UpdateUser(context.Background(), "user-123", UserPatch{Peran: &peran}, "admin-1")
	require.NoError(t, err)

	for _, e := range recorder.Entries() {
		sebelum, ok := e.Sebelum.(map[string]any)
		require.True(t, ok, e.Aksi)
		sesudah, ok := e.Sesudah.(map[string]any)
		require.True(t, ok, e.Aksi)
		assert.Contains(t, sebelum, "peran", e.Aksi)
		assert.Contains(t, sesudah, "peran", e.Aksi)
	}
}
