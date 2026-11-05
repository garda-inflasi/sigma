package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/pressly/goose/v3"
)

// versiEnkripsiMFASecret adalah versi migrasi data yang mengenkripsi pengguna.mfa_secret (slice 01-C).
// Berada setelah 000063 (kode cadangan) dan memakai kunci AES dari lingkungan, jadi ditulis dalam Go.
const versiEnkripsiMFASecret = 64

// migrasiGo adalah migrasi yang tidak bisa berupa SQL murni.
func migrasiGo() []*goose.Migration {
	return []*goose.Migration{
		goose.NewGoMigration(versiEnkripsiMFASecret,
			&goose.GoFunc{RunTx: enkripsiMFASecret},
			&goose.GoFunc{RunTx: dekripsiMFASecret},
		),
	}
}

type rahasiaMFA struct {
	id, nilai string
}

// enkripsiMFASecret menyegel setiap mfa_secret polos dengan kunci AES (AES_KEY). Baris yang sudah
// bersegel dilewati, jadi aman dijalankan ulang. Kunci hanya dibaca bila ada baris yang perlu disegel.
func enkripsiMFASecret(ctx context.Context, tx *sql.Tx) error {
	return ubahMFASecret(ctx, tx, func(ring *crypto.Keyring, nilai string) (string, bool, error) {
		if ring.IsSealed(nilai) {
			return "", false, nil
		}
		sealed, err := ring.Seal(nilai)
		return sealed, true, err
	})
}

// dekripsiMFASecret memulihkan mfa_secret polos dari yang bersegel (kebalikan enkripsiMFASecret);
// memerlukan kunci yang sama.
func dekripsiMFASecret(ctx context.Context, tx *sql.Tx) error {
	return ubahMFASecret(ctx, tx, func(ring *crypto.Keyring, nilai string) (string, bool, error) {
		if !ring.IsSealed(nilai) {
			return "", false, nil
		}
		plain, err := ring.Open(nilai)
		return plain, true, err
	})
}

func ubahMFASecret(ctx context.Context, tx *sql.Tx, ubah func(*crypto.Keyring, string) (string, bool, error)) error {
	rows, err := tx.QueryContext(ctx, "SELECT id, mfa_secret FROM pengguna WHERE mfa_secret IS NOT NULL AND mfa_secret <> '' ORDER BY id")
	if err != nil {
		return fmt.Errorf("baca mfa_secret: %w", err)
	}
	var semua []rahasiaMFA
	for rows.Next() {
		var r rahasiaMFA
		if err := rows.Scan(&r.id, &r.nilai); err != nil {
			_ = rows.Close()
			return fmt.Errorf("baca mfa_secret: %w", err)
		}
		semua = append(semua, r)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("baca mfa_secret: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("baca mfa_secret: %w", err)
	}
	if len(semua) == 0 {
		return nil
	}

	ring, err := crypto.KeyringFromEnv(os.Getenv)
	if err != nil {
		return fmt.Errorf("kunci enkripsi MFA: %w", err)
	}
	for _, r := range semua {
		baru, berubah, err := ubah(ring, r.nilai)
		if err != nil {
			return fmt.Errorf("ubah mfa_secret %s: %w", r.id, err)
		}
		if !berubah {
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE pengguna SET mfa_secret = $1 WHERE id = $2", baru, r.id); err != nil {
			return fmt.Errorf("simpan mfa_secret %s: %w", r.id, err)
		}
	}
	return nil
}
