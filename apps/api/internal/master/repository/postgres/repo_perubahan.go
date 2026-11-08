package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
)

var _ domain.RepositoryPerubahan = (*Repository)(nil)

// tabelEntitas memetakan entitas usulan ke tabel dan kolom kuncinya (nama tetap, bukan masukan pengguna).
func tabelEntitas(entitas string) (tabel, kunci string, err error) {
	switch entitas {
	case domain.EntitasKomoditas:
		return "komoditas", "kode", nil
	case domain.EntitasPasar:
		return "pasar", "kode", nil
	}
	return "", "", fmt.Errorf("%w: entitas %q tidak didukung", domain.ErrValidasi, entitas)
}

// snapshot membaca baris entitas sebagai objek JSON (angka utuh lewat json.Number) dan menguncinya untuk
// transaksi ini. nil bila baris tidak ada.
func snapshot(tx *gorm.DB, entitas, kode string) (map[string]any, error) {
	tabel, kunci, err := tabelEntitas(entitas)
	if err != nil {
		return nil, err
	}
	var raw []string
	q := fmt.Sprintf("SELECT to_jsonb(t)::text FROM %s t WHERE t.%s = ? FOR UPDATE", tabel, kunci)
	if err := tx.Raw(q, kode).Scan(&raw).Error; err != nil {
		return nil, fmt.Errorf("baca %s %s: %w", tabel, kode, err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw[0]))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("baca %s %s: %w", tabel, kode, err)
	}
	return out, nil
}

func versiDari(row map[string]any) int {
	if n, ok := row["versi"].(json.Number); ok {
		if v, err := n.Int64(); err == nil {
			return int(v)
		}
	}
	return 0
}

func teks(row map[string]any, kolom string) string {
	s, _ := row[kolom].(string)
	return s
}

// periksaRujukan memastikan satuan, kabupaten, dan kecamatan yang diusulkan ada dan konsisten.
// kabupatenKode adalah kabupaten pasar (dari usulan TAMBAH atau dari baris yang ada).
func periksaRujukan(tx *gorm.DB, entitas string, isi map[string]any, kabupatenKode string) error {
	ada := func(q string, args ...any) (bool, error) {
		var n int64
		if err := tx.Raw(q, args...).Scan(&n).Error; err != nil {
			return false, err
		}
		return n > 0, nil
	}
	if satuan, ok := isi["satuan"].(string); ok && entitas == domain.EntitasKomoditas {
		ok, err := ada("SELECT count(*) FROM satuan WHERE kode = ?", satuan)
		if err != nil {
			return fmt.Errorf("periksa satuan: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: satuan %q belum terdaftar", domain.ErrValidasi, satuan)
		}
	}
	if entitas != domain.EntitasPasar {
		return nil
	}
	if kab, ok := isi["kabupaten_kode"].(string); ok {
		ok, err := ada("SELECT count(*) FROM kabupaten WHERE kode = ?", kab)
		if err != nil {
			return fmt.Errorf("periksa kabupaten: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: kabupaten %q tidak dikenal", domain.ErrValidasi, kab)
		}
	}
	if kec, ok := isi["kecamatan_kode"].(string); ok {
		ok, err := ada("SELECT count(*) FROM kecamatan WHERE kode = ? AND kabupaten_kode = ?", kec, kabupatenKode)
		if err != nil {
			return fmt.Errorf("periksa kecamatan: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: kecamatan %q tidak ada di kabupaten %s", domain.ErrValidasi, kec, kabupatenKode)
		}
	}
	return nil
}

func jsonObjek(m map[string]any) (string, error) {
	if m == nil {
		m = map[string]any{}
	}
	raw, err := json.Marshal(m)
	return string(raw), err
}

func (r *Repository) Ajukan(ctx context.Context, u domain.UsulanBaru) (domain.PerubahanMaster, error) {
	var hasil domain.PerubahanMaster
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		kini, err := snapshot(tx, u.Entitas, u.EntitasID)
		if err != nil {
			return err
		}
		isi := map[string]any{}
		sebelum := map[string]any{}
		versi := 1
		kabupaten := ""
		switch u.Jenis {
		case domain.JenisTambah:
			if kini != nil {
				return domain.ErrKodeSudahAda
			}
			for k, v := range u.Perubahan {
				isi[k] = v
			}
			kabupaten, _ = isi["kabupaten_kode"].(string)
		case domain.JenisUbah, domain.JenisTinjau:
			if kini == nil {
				return domain.ErrEntitasTidakAda
			}
			if u.Entitas == domain.EntitasKomoditas {
				status := teks(kini, "status")
				if status == domain.StatusDiarsipkan || (u.Jenis == domain.JenisTinjau && status != domain.StatusAktif) {
					return domain.ErrStatusTidakSesuai
				}
			}
			for k, v := range u.Perubahan {
				if domain.SamaNilai(kini[k], v) {
					continue // bidang yang tidak berubah bukan bagian usulan
				}
				isi[k], sebelum[k] = v, kini[k]
			}
			if u.Jenis == domain.JenisUbah && len(isi) == 0 {
				return domain.ErrTanpaPerubahan
			}
			versi = versiDari(kini) + 1
			kabupaten = teks(kini, "kabupaten_kode")
		default:
			return fmt.Errorf("%w: jenis %q tidak didukung", domain.ErrValidasi, u.Jenis)
		}
		if err := periksaRujukan(tx, u.Entitas, isi, kabupaten); err != nil {
			return err
		}

		perubahanJSON, err := jsonObjek(isi)
		if err != nil {
			return err
		}
		sebelumJSON, err := jsonObjek(sebelum)
		if err != nil {
			return err
		}
		hasil = domain.PerubahanMaster{
			Entitas: u.Entitas, EntitasID: u.EntitasID, Jenis: u.Jenis, Versi: versi,
			Perubahan: perubahanJSON, Sebelum: sebelumJSON, Status: domain.UsulanMenunggu,
			Alasan: idx.PtrAtauNil(u.Alasan), DiajukanOlehID: u.AktorID,
		}
		if err := tx.Create(&hasil).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrUsulanMenunggu
			}
			return fmt.Errorf("simpan usulan: %w", err)
		}
		if u.Entitas == domain.EntitasKomoditas && u.Jenis != domain.JenisTambah {
			if err := tx.Table("komoditas").Where("kode = ?", u.EntitasID).
				Update("status", domain.StatusMenungguPersetujuan).Error; err != nil {
				return fmt.Errorf("tandai komoditas menunggu: %w", err)
			}
		}
		return nil
	})
	return hasil, err
}

func (r *Repository) Putuskan(ctx context.Context, id idx.ID, k domain.Keputusan) (domain.Putusan, error) {
	var hasil domain.Putusan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []domain.PerubahanMaster
		if err := tx.Raw("SELECT * FROM perubahan_master WHERE id = ? FOR UPDATE", id).Scan(&rows).Error; err != nil {
			return fmt.Errorf("baca usulan: %w", err)
		}
		if len(rows) == 0 {
			return domain.ErrUsulanTidakAda
		}
		var err error
		hasil, err = putuskan(tx, rows[0], k)
		return err
	})
	return hasil, err
}

func (r *Repository) PutuskanMenunggu(ctx context.Context, entitas, entitasID string, k domain.Keputusan) (domain.Putusan, error) {
	var hasil domain.Putusan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []domain.PerubahanMaster
		err := tx.Raw(`SELECT * FROM perubahan_master WHERE entitas = ? AND entitas_id = ? AND status = 'MENUNGGU' FOR UPDATE`,
			entitas, entitasID).Scan(&rows).Error
		if err != nil {
			return fmt.Errorf("baca usulan: %w", err)
		}
		if len(rows) == 0 {
			return domain.ErrUsulanTidakAda
		}
		hasil, err = putuskan(tx, rows[0], k)
		return err
	})
	return hasil, err
}

// putuskan menerapkan keputusan pada usulan yang sudah dikunci transaksi tx.
func putuskan(tx *gorm.DB, p domain.PerubahanMaster, k domain.Keputusan) (domain.Putusan, error) {
	if p.Status != domain.UsulanMenunggu {
		return domain.Putusan{}, domain.ErrBukanMenunggu
	}
	switch k.Aksi {
	case domain.KeputusanSetujui, domain.KeputusanTolak:
		if k.AktorID == p.DiajukanOlehID {
			return domain.Putusan{}, domain.ErrPemisahanTugas
		}
	case domain.KeputusanBatalkan:
		if k.AktorID != p.DiajukanOlehID {
			return domain.Putusan{}, domain.ErrBukanPengaju
		}
	default:
		return domain.Putusan{}, fmt.Errorf("%w: keputusan %q tidak dikenal", domain.ErrValidasi, k.Aksi)
	}

	sekarang := time.Now()
	if k.Aksi != domain.KeputusanSetujui {
		alasan := k.Alasan
		if k.Aksi == domain.KeputusanBatalkan {
			alasan = "Dibatalkan pengaju"
			if strings.TrimSpace(k.Alasan) != "" {
				alasan += ": " + strings.TrimSpace(k.Alasan)
			}
		}
		err := tx.Model(&domain.PerubahanMaster{}).Where("id = ?", p.ID).Updates(map[string]any{
			"status": domain.UsulanDitolak, "ditolak_oleh_id": k.AktorID, "ditolak_pada": sekarang, "alasan_penolakan": alasan,
		}).Error
		if err != nil {
			return domain.Putusan{}, fmt.Errorf("tolak usulan: %w", err)
		}
		if p.Entitas == domain.EntitasKomoditas {
			err := tx.Table("komoditas").
				Where("kode = ? AND status = ?", p.EntitasID, domain.StatusMenungguPersetujuan).
				Update("status", domain.StatusAktif).Error
			if err != nil {
				return domain.Putusan{}, fmt.Errorf("kembalikan status komoditas: %w", err)
			}
		}
		p.Status, p.DitolakOlehID, p.DitolakPada, p.AlasanPenolakan = domain.UsulanDitolak, &k.AktorID, &sekarang, &alasan
		return domain.Putusan{Usulan: p}, nil
	}

	// Disetujui: riwayat versi komoditas (pemicu, migrasi 000121) mencatat usulan yang menghasilkannya.
	if err := tx.Exec("SELECT set_config('garda.perubahan_master_id', ?, true), set_config('garda.aktor_id', ?, true)",
		p.ID.String(), k.AktorID).Error; err != nil {
		return domain.Putusan{}, fmt.Errorf("tandai usulan pada transaksi: %w", err)
	}
	var isi map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(p.Perubahan)))
	dec.UseNumber()
	if err := dec.Decode(&isi); err != nil {
		return domain.Putusan{}, fmt.Errorf("baca isi usulan %s: %w", p.ID, err)
	}
	isi = domain.KonversiTersimpan(p.Entitas, isi)
	tabel, kunci, err := tabelEntitas(p.Entitas)
	if err != nil {
		return domain.Putusan{}, err
	}
	kini, err := snapshot(tx, p.Entitas, p.EntitasID)
	if err != nil {
		return domain.Putusan{}, err
	}

	switch p.Jenis {
	case domain.JenisTambah:
		if kini != nil {
			return domain.Putusan{}, domain.ErrKodeSudahAda
		}
		kab, _ := isi["kabupaten_kode"].(string)
		if err := periksaRujukan(tx, p.Entitas, isi, kab); err != nil {
			return domain.Putusan{}, err
		}
		baris := map[string]any{kunci: p.EntitasID, "versi": 1}
		for b, v := range isi {
			baris[b] = v
		}
		if p.Entitas == domain.EntitasKomoditas {
			baris["status"] = domain.StatusAktif
		}
		if err := tx.Table(tabel).Create(baris).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.Putusan{}, domain.ErrKodeSudahAda
			}
			return domain.Putusan{}, fmt.Errorf("tambah %s %s: %w", tabel, p.EntitasID, err)
		}
	default:
		if kini == nil || versiDari(kini)+1 != p.Versi ||
			(p.Entitas == domain.EntitasKomoditas && teks(kini, "status") == domain.StatusDiarsipkan) {
			return domain.Putusan{}, domain.ErrKedaluwarsa
		}
		if err := periksaRujukan(tx, p.Entitas, isi, teks(kini, "kabupaten_kode")); err != nil {
			return domain.Putusan{}, err
		}
		ubah := map[string]any{"versi": p.Versi}
		for b, v := range isi {
			if b == "kabupaten_kode" || b == kunci {
				continue // tidak pernah diubah lewat UBAH
			}
			ubah[b] = v
		}
		if p.Entitas == domain.EntitasKomoditas {
			ubah["status"] = domain.StatusAktif
		}
		if err := tx.Table(tabel).Where(kunci+" = ?", p.EntitasID).Updates(ubah).Error; err != nil {
			return domain.Putusan{}, fmt.Errorf("terapkan usulan pada %s %s: %w", tabel, p.EntitasID, err)
		}
	}

	err = tx.Model(&domain.PerubahanMaster{}).Where("id = ?", p.ID).Updates(map[string]any{
		"status": domain.UsulanDisetujui, "disetujui_oleh_id": k.AktorID, "disetujui_pada": sekarang,
	}).Error
	if err != nil {
		return domain.Putusan{}, fmt.Errorf("setujui usulan: %w", err)
	}
	sesudah, err := snapshot(tx, p.Entitas, p.EntitasID)
	if err != nil {
		return domain.Putusan{}, err
	}
	p.Status, p.DisetujuiOlehID, p.DisetujuiPada = domain.UsulanDisetujui, &k.AktorID, &sekarang
	return domain.Putusan{Usulan: p, Sebelum: kini, Sesudah: sesudah}, nil
}

func (r *Repository) ArsipkanKomoditas(ctx context.Context, kode, aktorID string) (bool, error) {
	berubah := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("komoditas").
			Where("kode = ? AND status IN ?", kode, []string{domain.StatusAktif, domain.StatusMenungguPersetujuan}).
			Update("status", domain.StatusDiarsipkan)
		if res.Error != nil {
			return fmt.Errorf("arsipkan komoditas %s: %w", kode, res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		berubah = true
		// Usulan yang masih menunggu ikut ditutup: komoditas arsip tidak bisa diubah lagi.
		err := tx.Model(&domain.PerubahanMaster{}).
			Where("entitas = ? AND entitas_id = ? AND status = ?", domain.EntitasKomoditas, kode, domain.UsulanMenunggu).
			Updates(map[string]any{
				"status": domain.UsulanDitolak, "ditolak_oleh_id": aktorID, "ditolak_pada": time.Now(),
				"alasan_penolakan": "Komoditas diarsipkan",
			}).Error
		if err != nil {
			return fmt.Errorf("tutup usulan komoditas %s: %w", kode, err)
		}
		return nil
	})
	return berubah, err
}

// viewPerubahan memilih usulan beserta nama entitas (nama kini, atau nama usulan untuk TAMBAH) dan pelakunya.
const viewPerubahan = `
	SELECT p.*,
	       COALESCE(CASE p.entitas
	                  WHEN 'komoditas' THEN (SELECT k.nama FROM komoditas k WHERE k.kode = p.entitas_id OR k.id::text = p.entitas_id)
	                  WHEN 'pasar' THEN (SELECT s.nama FROM pasar s WHERE s.kode = p.entitas_id OR s.id::text = p.entitas_id)
	                END, p.perubahan->>'nama') AS entitas_nama,
	       ua.nama AS diajukan_nama, us.nama AS disetujui_nama, ut.nama AS ditolak_nama
	  FROM perubahan_master p
	  LEFT JOIN pengguna ua ON ua.id = p.diajukan_oleh_id
	  LEFT JOIN pengguna us ON us.id = p.disetujui_oleh_id
	  LEFT JOIN pengguna ut ON ut.id = p.ditolak_oleh_id`

func (r *Repository) GetPerubahan(ctx context.Context, id idx.ID) (domain.PerubahanView, error) {
	var rows []domain.PerubahanView
	if err := r.db.WithContext(ctx).Raw(viewPerubahan+" WHERE p.id = ?", id).Scan(&rows).Error; err != nil {
		return domain.PerubahanView{}, fmt.Errorf("baca usulan: %w", err)
	}
	if len(rows) == 0 {
		return domain.PerubahanView{}, domain.ErrUsulanTidakAda
	}
	return rows[0], nil
}

func (r *Repository) ListPerubahan(ctx context.Context, f domain.FilterPerubahan) ([]domain.PerubahanView, int64, error) {
	var where []string
	var args []any
	if f.Status != "" {
		where, args = append(where, "p.status = ?"), append(args, f.Status)
	}
	if f.Entitas != "" {
		where, args = append(where, "p.entitas = ?"), append(args, f.Entitas)
	}
	if f.EntitasID != "" {
		where, args = append(where, "p.entitas_id = ?"), append(args, f.EntitasID)
	}
	kondisi := ""
	if len(where) > 0 {
		kondisi = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT count(*) FROM perubahan_master p"+kondisi, args...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("hitung usulan: %w", err)
	}
	// Antrean menunggu: terlama lebih dulu (dilayani berurutan); riwayat: terbaru lebih dulu.
	urut := " ORDER BY p.dibuat_pada DESC, p.id DESC"
	if f.Status == domain.UsulanMenunggu {
		urut = " ORDER BY p.dibuat_pada, p.id"
	}
	q := viewPerubahan + kondisi + urut
	if f.Limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.Limit, (max(f.Page, 1)-1)*f.Limit)
	}
	var rows []domain.PerubahanView
	if err := r.db.WithContext(ctx).Raw(q, args...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("daftar usulan: %w", err)
	}
	return rows, total, nil
}
