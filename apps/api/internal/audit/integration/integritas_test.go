package integration

// Integritas audit (03-C): rantai hash yang dihitung pemicu, verifikasi di Go, partisi bulanan, dan retensi ke arsip.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/arsip"
	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func (s *suite) semuaBaris(t *testing.T) []domain.AuditLog {
	t.Helper()
	var rows []domain.AuditLog
	require.NoError(t, s.db.Order("id").Find(&rows).Error)
	return rows
}

// rusakkan menjalankan perubahan langsung pada audit_log dengan pemicu dimatikan, seperti penyerang yang memegang
// hak superuser.
func (s *suite) rusakkan(t *testing.T, sql string, args ...any) {
	t.Helper()
	require.NoError(t, s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL session_replication_role = replica").Error; err != nil {
			return err
		}
		return tx.Exec(sql, args...).Error
	}))
}

func TestRantai_HashGoSamaDenganPemicuDanSQL(t *testing.T) {
	s := newSuite(t, "audit_rantai_sama")
	ctx := t.Context()
	require.NoError(t, s.writer.Write(ctx, audit.Entry{
		AktorID: uid("admin-1"), Aksi: "UBAH", Entitas: "uji", EntitasID: "ñ-漢字", Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"b": 1.5, "a": "é \"kutip\" \n", "z": []any{nil, true}}, Sesudah: map[string]any{},
		Metadata: map[string]any{"angka": 12345678901234, "kosong": ""},
	}))
	require.NoError(t, s.writer.Write(ctx, audit.Entry{Aksi: "TANPA_AKTOR", Entitas: "uji", EntitasID: "", Hasil: audit.HasilGagal}))
	require.NoError(t, s.db.Exec(`INSERT INTO audit_log (waktu, kode, aksi, entitas, entitas_id, hasil, ip, user_agent, correlation_id)
		VALUES (date_trunc('second', now()) + interval '1 microsecond', 'kode-lama', 'SQL', 'uji', '9', 'DITOLAK', '::1', 'agen', 'req-1')`).Error)

	rows := s.semuaBaris(t)
	require.Len(t, rows, 3)
	var sebelum []byte
	for _, row := range rows {
		assert.Equal(t, sebelum, row.HashSebelumnya, "baris %s menyambung ke baris sebelumnya", row.ID)
		assert.Equal(t, row.Hash, domain.HitungHash(row.HashSebelumnya, row), "hash Go sama dengan pemicu (baris %s)", row.ID)

		var dariSQL []byte
		require.NoError(t, s.db.Raw(`SELECT audit_log_hitung_hash(hash_sebelumnya, id, waktu, kode, aktor_nama, aktor_email,
			aktor_peran, aksi, entitas, entitas_id, hasil, ip, user_agent, correlation_id, sebelum, sesudah, metadata)
			FROM audit_log WHERE id = ?`, row.ID).Row().Scan(&dariSQL))
		assert.Equal(t, row.Hash, dariSQL)
		sebelum = row.Hash
	}
}

func TestRantai_PenulisSerentakTetapSatuRantai(t *testing.T) {
	s := newSuite(t, "audit_rantai_serentak")
	const penulis, perPenulis = 8, 15
	var wg sync.WaitGroup
	galat := make(chan error, penulis*perPenulis)
	for p := range penulis {
		wg.Go(func() {
			for i := range perPenulis {
				galat <- s.writer.Write(t.Context(), audit.Entry{
					AktorID: uid("admin-1"), Aksi: "SERENTAK", Entitas: "uji", EntitasID: fmt.Sprintf("%d-%d", p, i),
					Hasil: audit.HasilBerhasil,
				})
			}
		})
	}
	wg.Wait()
	close(galat)
	for err := range galat {
		require.NoError(t, err)
	}

	hasil, err := audit.NewIntegritas(s.db).Verifikasi(t.Context(), domain.PemicuManual)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusValid, hasil.Status)
	assert.EqualValues(t, penulis*perPenulis, hasil.JumlahBaris)
	assert.Zero(t, hasil.JumlahMasalah)
}

func TestVerifikasi_MendeteksiPerusakanDanTampilDiAPI(t *testing.T) {
	s := newSuite(t, "audit_verifikasi_rusak")
	token := s.login(t, adminEmail) // login ikut dicatat: baris pertama rantai
	for i := range 6 {
		require.NoError(t, s.writer.Write(t.Context(), audit.Entry{
			AktorID: uid("admin-1"), Aksi: "UJI", Entitas: "uji", EntitasID: fmt.Sprint(i), Hasil: audit.HasilBerhasil,
		}))
	}
	rows := s.semuaBaris(t)
	require.Len(t, rows, 7)
	integ := audit.NewIntegritas(s.db)

	// Sebelum verifikasi pertama: belum ada status.
	w := s.call(t, http.MethodGet, "/v1/audit/integritas", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var awal struct {
		Data struct {
			VerifikasiTerakhir *struct{} `json:"verifikasi_terakhir"`
			Partisi            []struct {
				Nama string `json:"nama"`
			} `json:"partisi"`
			Arsip []struct{} `json:"arsip"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &awal))
	assert.Nil(t, awal.Data.VerifikasiTerakhir)
	assert.NotEmpty(t, awal.Data.Partisi)
	assert.NotNil(t, awal.Data.Arsip, "arsip kosong dikirim sebagai [] bukan null")

	hasil, err := integ.Verifikasi(t.Context(), domain.PemicuManual)
	require.NoError(t, err)
	require.Equal(t, domain.StatusValid, hasil.Status)

	// Isi baris ke-3 diubah langsung (melewati pemicu append-only).
	s.rusakkan(t, `UPDATE audit_log SET hasil = 'GAGAL' WHERE id = ?`, rows[2].ID)
	hasil, err = integ.Verifikasi(t.Context(), domain.PemicuJadwal)
	require.NoError(t, err)
	require.Equal(t, domain.StatusRusak, hasil.Status)
	assert.EqualValues(t, 1, hasil.JumlahMasalah)

	w = s.call(t, http.MethodGet, "/v1/audit/integritas", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			VerifikasiTerakhir struct {
				Status         string `json:"status"`
				Pemicu         string `json:"pemicu"`
				JumlahBaris    int64  `json:"jumlah_baris"`
				HashAkhir      string `json:"hash_akhir"`
				MasalahPertama struct {
					ID    string `json:"id"`
					Jenis string `json:"jenis"`
				} `json:"masalah_pertama"`
			} `json:"verifikasi_terakhir"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	v := body.Data.VerifikasiTerakhir
	assert.Equal(t, domain.StatusRusak, v.Status)
	assert.Equal(t, domain.PemicuJadwal, v.Pemicu)
	assert.EqualValues(t, len(rows), v.JumlahBaris)
	assert.Len(t, v.HashAkhir, 64)
	assert.Equal(t, rows[2].ID.String(), v.MasalahPertama.ID)
	assert.Equal(t, domain.MasalahIsiBerubah, v.MasalahPertama.Jenis)

	temuan := s.rows(t, "VERIFIKASI_AUDIT")
	require.Len(t, temuan, 1, "temuan perusakan dicatat ke audit trail")
	assert.Equal(t, audit.HasilGagal, temuan[0].Hasil)

	// Baris ke-5 dihapus: baris sesudahnya tidak lagi menyambung.
	s.rusakkan(t, `DELETE FROM audit_log WHERE id = ?`, rows[4].ID)
	hasil, err = integ.Verifikasi(t.Context(), domain.PemicuManual)
	require.NoError(t, err)
	masalah := map[string]string{}
	for _, m := range usecaseMasalah(hasil) {
		masalah[fmt.Sprint(m.ID)] = m.Jenis
	}
	assert.Equal(t, domain.MasalahIsiBerubah, masalah[rows[2].ID.String()])
	assert.Equal(t, domain.MasalahRantaiPutus, masalah[rows[5].ID.String()])

	// Peran tanpa audit.lihat tidak boleh melihat status integritas.
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/audit/integritas", s.login(t, viewerEmail), nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/audit/integritas", "", nil).Code)
}

func usecaseMasalah(v domain.Verifikasi) []domain.Masalah {
	var out []domain.Masalah
	if v.Masalah != nil {
		_ = json.Unmarshal([]byte(*v.Masalah), &out)
	}
	return out
}

func TestWriter_MembuatPartisiBulanYangBelumAda(t *testing.T) {
	s := newSuite(t, "audit_partisi_otomatis")
	nanti := time.Now().UTC().AddDate(0, 6, 0)
	nama := domain.NamaPartisi(nanti)
	var ada bool
	require.NoError(t, s.db.Raw(`SELECT to_regclass(?) IS NOT NULL`, nama).Scan(&ada).Error)
	require.False(t, ada, "migrasi hanya menyiapkan tiga bulan ke depan")

	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{Aksi: "TERJADWAL", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil, Waktu: nanti}))
	require.NoError(t, s.db.Raw(`SELECT to_regclass(?) IS NOT NULL`, nama).Scan(&ada).Error)
	assert.True(t, ada)

	// Job worker menyiapkan jendela ke depan; idempoten.
	integ := audit.NewIntegritas(s.db)
	dibuat, err := integ.SiapkanPartisi(t.Context(), 4)
	require.NoError(t, err)
	assert.NotContains(t, dibuat, domain.NamaPartisi(time.Now()), "bulan ini sudah ada")
	dibuat, err = integ.SiapkanPartisi(t.Context(), 4)
	require.NoError(t, err)
	assert.Empty(t, dibuat)

	// Partisi tidak bisa di-TRUNCATE langsung.
	err = s.db.Exec(`TRUNCATE ` + nama).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "append-only")
}

func TestKode_TetapUnikLintasPartisi(t *testing.T) {
	s := newSuite(t, "audit_kode_unik")
	require.NoError(t, s.db.Exec(`SELECT audit_log_siapkan_partisi(now() - interval '70 days', 4)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO audit_log (waktu, kode, aksi, entitas, entitas_id, hasil) VALUES (now(), 'K-1', 'A', 'e', '1', 'BERHASIL')`).Error)
	err := s.db.Exec(`INSERT INTO audit_log (waktu, kode, aksi, entitas, entitas_id, hasil) VALUES (now() - interval '65 days', 'K-1', 'A', 'e', '1', 'BERHASIL')`).Error
	require.Error(t, err, "kode lama unik di seluruh partisi, bukan hanya per waktu")
	assert.Contains(t, err.Error(), "sudah dipakai")
}

func TestRetensi_MengarsipkanPartisiLamaDanRantaiTetapTerverifikasi(t *testing.T) {
	s := newSuite(t, "audit_retensi")
	tulis := func(aksi string, waktu time.Time) {
		t.Helper()
		require.NoError(t, s.writer.Write(t.Context(), audit.Entry{
			AktorID: uid("admin-1"), Aksi: aksi, Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil,
			Sebelum: map[string]any{"a": 1}, Waktu: waktu,
		}))
	}
	maret := time.Date(2024, 3, 5, 10, 0, 0, 0, time.UTC)
	tulis("LAMA_1", maret) // partisi audit_log_p202403 dibuat penulis
	tulis("LAMA_2", maret.Add(time.Hour))
	tulis("BARU_1", time.Now())
	tulis("LAMA_3", maret.Add(2*time.Hour)) // bertanggal mundur: id-nya menyelang di antara baris baru
	tulis("BARU_2", time.Now())

	integ := audit.NewIntegritas(s.db)
	dir := t.TempDir()
	hasil, err := integ.Retensi(t.Context(), audit.KonfigRetensi{Bulan: 12, Dir: dir})
	require.NoError(t, err)
	require.Len(t, hasil, 1)
	assert.Equal(t, "audit_log_p202403", hasil[0].Partisi)
	assert.EqualValues(t, 3, hasil[0].JumlahBaris)

	var ada bool
	require.NoError(t, s.db.Raw(`SELECT to_regclass('audit_log_p202403') IS NOT NULL`).Scan(&ada).Error)
	assert.False(t, ada, "partisi dilepas dari basis data")
	var sisa int64
	require.NoError(t, s.db.Table("audit_log").Where("aksi LIKE 'LAMA_%'").Count(&sisa).Error)
	assert.Zero(t, sisa)

	// Berkas arsip utuh dan bisa diverifikasi tanpa basis data; dua segmen karena LAMA_3 menyelang.
	manifest := filepath.Join(dir, arsip.NamaBerkasManifest("audit_log_p202403"))
	cek, err := arsip.Verifikasi(manifest)
	require.NoError(t, err)
	assert.True(t, cek.Valid(), "%+v", cek.Masalah)
	assert.EqualValues(t, 3, cek.JumlahBaris)
	assert.Equal(t, 2, cek.JumlahSegmen)

	// Rantai yang tersisa tetap terverifikasi utuh lewat segmen arsip.
	v, err := integ.Verifikasi(t.Context(), domain.PemicuManual)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusValid, v.Status, "%v", usecaseMasalah(v))
	assert.EqualValues(t, 2, v.JumlahSegmen)
	jumlahHidup := v.JumlahBaris

	// Arsip dan pelepasannya tercatat; putaran berikutnya tidak mengarsipkan apa pun lagi.
	require.Len(t, s.rows(t, "ARSIP_AUDIT"), 1)
	hasil, err = integ.Retensi(t.Context(), audit.KonfigRetensi{Bulan: 12, Dir: dir})
	require.NoError(t, err)
	assert.Empty(t, hasil)

	// Bulan yang sudah diarsipkan tidak bisa dibuka lagi.
	err = s.writer.Write(t.Context(), audit.Entry{Aksi: "LAMA_4", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil, Waktu: maret})
	assert.Error(t, err)

	// Catatan arsip append-only, dan manifest yang hilang dilengkapi lagi.
	assert.Error(t, s.db.Exec(`UPDATE audit_log_arsip SET berkas = 'lain'`).Error)
	require.NoError(t, os.Remove(manifest))
	_, err = integ.Retensi(t.Context(), audit.KonfigRetensi{Bulan: 12, Dir: dir})
	require.NoError(t, err)
	_, err = os.Stat(manifest)
	assert.NoError(t, err)

	// Status di API memuat arsipnya.
	w := s.call(t, http.MethodGet, "/v1/audit/integritas", s.login(t, adminEmail), nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, bytes.Contains(w.Body.Bytes(), []byte(`"partisi":"audit_log_p202403"`)), w.Body.String())

	// Verifikasi akhir: login di atas menambah baris, rantai tetap utuh.
	v, err = integ.Verifikasi(t.Context(), domain.PemicuManual)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusValid, v.Status)
	assert.Greater(t, v.JumlahBaris, jumlahHidup)
}

func TestRetensi_MenolakKebijakanTerlaluPendek(t *testing.T) {
	s := newSuite(t, "audit_retensi_pendek")
	_, err := audit.NewIntegritas(s.db).Retensi(t.Context(), audit.KonfigRetensi{Bulan: 6, Dir: t.TempDir()})
	assert.Error(t, err)

	// Fungsi basis data menolak juga, walau dipanggil langsung dengan bukti apa pun.
	err = s.db.Exec(`SELECT audit_log_lepas_partisi('audit_log_p202001', 6, 0, sha256(''::bytea), 'x', repeat('a', 64))`).Error
	assert.Error(t, err)
}
