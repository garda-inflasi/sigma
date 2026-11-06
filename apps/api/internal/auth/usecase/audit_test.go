package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLogin_SuccessWritesExactlyOneAuditEntry(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user := activeUser()
	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)
	repo.On("TouchLastLogin", mock.Anything, "user-123", mock.Anything).Return(nil)

	_, err := uc.Login(context.Background(), "test@example.com", "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "LOGIN", entries[0].Aksi)
	assert.Equal(t, "user-123", entries[0].AktorID)
	assert.Equal(t, audit.HasilBerhasil, entries[0].Hasil)
}

func TestLogin_FailuresAreAudited(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(repo *mockRepository)
		password   string
		wantActor  string
		wantAlasan string
	}{
		{
			name: "email tak dikenal",
			setup: func(repo *mockRepository) {
				repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(nil, gorm.ErrRecordNotFound)
				repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
			},
			password:   "Test1234!",
			wantAlasan: "user_not_found",
		},
		{
			name: "password salah",
			setup: func(repo *mockRepository) {
				repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(activeUser(), nil)
				repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
			},
			password:   "Salah123!",
			wantActor:  "user-123",
			wantAlasan: "wrong_password",
		},
		{
			name: "akun terkunci",
			setup: func(repo *mockRepository) {
				repo.kunciTerkunci("test@example.com")
				repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(activeUser(), nil)
			},
			password:   "Test1234!",
			wantActor:  "user-123",
			wantAlasan: "account_locked",
		},
		{
			name: "akun nonaktif",
			setup: func(repo *mockRepository) {
				inactive := activeUser()
				inactive.Status = "NONAKTIF"
				repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(inactive, nil)
				repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
			},
			password:   "Test1234!",
			wantActor:  "user-123",
			wantAlasan: "account_inactive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)
			tc.setup(repo)

			_, err := uc.Login(context.Background(), "test@example.com", tc.password, "10.0.0.1")

			require.Error(t, err)
			entries := recorder.Entries()
			require.Len(t, entries, 1)
			assert.Equal(t, "LOGIN", entries[0].Aksi)
			assert.Equal(t, audit.HasilGagal, entries[0].Hasil)
			assert.Equal(t, tc.wantActor, entries[0].AktorID)
			assert.Equal(t, tc.wantAlasan, entries[0].Metadata["alasan"])
		})
	}
}

func TestLogout_WritesAuditEntry(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(nil)

	require.NoError(t, uc.Logout(context.Background(), "user-123", "jti-1"))

	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "LOGOUT", entry.Aksi)
	assert.Equal(t, "user-123", entry.AktorID)
}

func TestAssignRole_AuditsBeforeAndAfter(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user := activeUser()
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	_, err := uc.AssignRole(context.Background(), "user-123", "MANAJER_DATA", "admin-1")

	require.NoError(t, err)
	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "ASSIGN_ROLE", entries[0].Aksi)
	assert.Equal(t, "admin-1", entries[0].AktorID)
	assert.Equal(t, "OPS_MAHASISWA", entries[0].Sebelum.(map[string]any)["peran"])
	assert.Equal(t, "MANAJER_DATA", entries[0].Sesudah.(map[string]any)["peran"])
}

func TestUpdateUser_AuditsBeforeAndAfter(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user := activeUser()
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	nama := "Nama Baru"

	_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Nama: &nama}, "admin-1")

	require.NoError(t, err)
	entry, _ := recorder.Last()
	assert.Equal(t, "UPDATE_USER", entry.Aksi)
	assert.Equal(t, "Test User", entry.Sebelum.(map[string]any)["nama"])
	assert.Equal(t, "Nama Baru", entry.Sesudah.(map[string]any)["nama"])
}

func TestCreateUser_AuditsActorAndMarksMandatoryPasswordChange(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByEmail", mock.Anything, "new@example.com").Return(nil, gorm.ErrRecordNotFound)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	user := &domain.Pengguna{ID: "new-user", Email: "new@example.com", Peran: "OPS_MAHASISWA"}

	require.NoError(t, uc.CreateUser(context.Background(), user, "NewUser1!", "admin-1"))

	assert.True(t, user.WajibGantiPassword, "kata sandi ditetapkan admin: wajib diganti")
	entry, _ := recorder.Last()
	assert.Equal(t, "CREATE_USER", entry.Aksi)
	assert.Equal(t, "admin-1", entry.AktorID)
}

func TestBulkUserAction_AuditsActorAndIDs(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("BulkUpdateUsers", mock.Anything, []string{"a", "b"}, domain.BulkNonaktifkan).Return(int64(2), nil)

	_, err := uc.BulkUserAction(context.Background(), []string{"a", "b"}, "nonaktifkan", "admin-1")

	require.NoError(t, err)
	entry, _ := recorder.Last()
	assert.Equal(t, "BULK_nonaktifkan", entry.Aksi)
	assert.Equal(t, "admin-1", entry.AktorID)
	assert.Equal(t, []string{"a", "b"}, entry.Metadata["ids"])
}

func TestAuditFailureDoesNotFailLogout(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	recorder.Err = errors.New("db mati")
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	assert.NoError(t, uc.Logout(context.Background(), "user-123", ""))
}
