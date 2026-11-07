// Package seeder mengisi definisi tahap intervensi (referensi, aman di produksi)
// dan kartu kanban contoh (demo, hanya pengembangan). Komoditas dan kabupaten
// berasal dari master.reference; pengguna dari auth.demo; alert dari ews.demo.
//
// ID kartu adalah UUID stabil idx.UUIDStabil("intervensi", "<kode>") (sama dengan migrasi 000112), jadi basis
// data baru dan basis data hasil migrasi memakai UUID yang sama dan seed tidak menggandakan baris. Kode
// tampilan INT-n disimpan di kolom kode.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Tahap, Alur, LangkahAlur int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Kartu, PenanggungJawab, Riwayat, Persetujuan int
}

const day = 24 * time.Hour

// SeedReference meng-upsert definisi tahap tiga siklus dan menambahkan alur persetujuan
// awal yang belum ada (yang sudah ada tidak diubah). Idempoten.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	tahap := BuildTahap()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "siklus"}, {Name: "kode_tahap"}},
		DoUpdates: clause.AssignmentColumns([]string{"urutan", "label", "petunjuk", "izin_dibutuhkan", "sla_hari"}),
	}).Create(&tahap).Error
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed tahap intervensi: %w", err)
	}
	alur, langkah, err := seedAlur(ctx, db)
	if err != nil {
		return ReferenceSummary{}, err
	}
	return ReferenceSummary{Tahap: len(tahap), Alur: alur, LangkahAlur: langkah}, nil
}

// SeedDemo meng-upsert kartu contoh dan mengembalikan penanggung jawabnya ke
// keadaan awal; tenggat relatif terhadap now. Hanya untuk demo.
//
// Prasyarat: SeedReference, master.reference, auth.demo, dan ews.demo sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	kartu, pj := BuildKartu(now)
	riwayat := BuildRiwayat(kartu, pj)
	ids := make([]string, len(kartu))
	for i, k := range kartu {
		ids[i] = k.ID
	}

	var persetujuan int
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := isiAlertKartu(tx, kartu); err != nil {
			return err
		}
		// id adalah UUID stabil dari kode, jadi ON CONFLICT (id) aman (bukan kolom identity).
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"kode", "siklus", "tahap", "urutan", "judul", "deskripsi", "komoditas_kode", "kabupaten_kode", "prioritas",
				"tenggat", "rekomendasi_ai", "checklist_selesai", "checklist_total", "ews_alert_id", "dibuat_oleh_id",
				"dibuat_pada", "selesai_pada", "versi", "dieskalasi_pada", "dieskalasi_oleh_id", "tahap_sejak",
			}),
		}).Create(&kartu).Error
		if err != nil {
			return fmt.Errorf("seed kartu: %w", err)
		}
		if err := tx.Where("intervensi_id IN ?", ids).Delete(&domain.IntervensiPJ{}).Error; err != nil {
			return fmt.Errorf("reset penanggung jawab: %w", err)
		}
		if err := tx.Create(&pj).Error; err != nil {
			return fmt.Errorf("seed penanggung jawab: %w", err)
		}
		// Riwayat kartu demo dibangun ulang (id baris dari sequence, jadi tidak di-upsert).
		if err := tx.Where("intervensi_id IN ?", ids).Delete(&domain.IntervensiRiwayat{}).Error; err != nil {
			return fmt.Errorf("reset riwayat kartu: %w", err)
		}
		if err := tx.Create(&riwayat).Error; err != nil {
			return fmt.Errorf("seed riwayat kartu: %w", err)
		}
		persetujuan, err = seedPermintaanDemo(tx, ids, now)
		return err
	})
	if err != nil {
		return DemoSummary{}, err
	}
	return DemoSummary{Kartu: len(kartu), PenanggungJawab: len(pj), Riwayat: len(riwayat), Persetujuan: persetujuan}, nil
}

// isiAlertKartu mengisi EwsAlertID kartu dari kode alert pemicunya (BuildKartu tidak tahu ID basis data).
// Alert harus sudah ada (ews.demo).
func isiAlertKartu(tx *gorm.DB, kartu []domain.Intervensi) error {
	var rows []struct {
		ID   idx.ID
		Kode string
	}
	if err := tx.Raw("SELECT id, kode FROM ews_alert WHERE kode IS NOT NULL").Scan(&rows).Error; err != nil {
		return fmt.Errorf("baca alert ews: %w", err)
	}
	peta := make(map[string]idx.ID, len(rows))
	for _, r := range rows {
		peta[r.Kode] = r.ID
	}
	for i := range kartu {
		kode := AlertKodeKartu(kartu[i])
		if kode == "" {
			continue
		}
		id, ok := peta[kode]
		if !ok {
			return fmt.Errorf("seed kartu: alert %s belum ada (jalankan ews.demo lebih dulu)", kode)
		}
		kartu[i].EwsAlertID = &id
	}
	return nil
}

// AlertKodeKartu mengembalikan kode alert EWS (EWS-n) pemicu kartu contoh itu menurut Kode kartu; kosong bila
// kartu tidak punya alert.
func AlertKodeKartu(k domain.Intervensi) string {
	if k.Kode == nil {
		return ""
	}
	for i, s := range kartuSeeds {
		if kodeKartu(i) == *k.Kode {
			return s.alert
		}
	}
	return ""
}

// kodeKartu adalah kode tampilan kartu contoh ke-i: INT-<301 + i>.
func kodeKartu(i int) string { return fmt.Sprintf("INT-%d", 301+i) }

// BuildTahap menyusun baris tahap_intervensi_def; urutan dihitung per siklus.
func BuildTahap() []domain.TahapDef {
	out := make([]domain.TahapDef, len(tahapSeeds))
	urutan := map[string]int{}
	for i, t := range tahapSeeds {
		urutan[t.siklus]++
		izin := t.izin
		if izin == "" {
			izin = domain.IzinBawaan
		}
		def := domain.TahapDef{
			Siklus: t.siklus, KodeTahap: t.kode, Urutan: urutan[t.siklus], Label: t.label,
			Petunjuk: t.petunjuk, IzinDibutuhkan: izin,
		}
		if t.slaHari > 0 {
			sla := t.slaHari
			def.SLAHari = &sla
		}
		out[i] = def
	}
	return out
}

// BuildKartu menyusun kartu dan penanggung jawabnya. Urutan kartu dalam satu
// tahap mengikuti urutan kemunculan; kartu di tahap penutup berstatus selesai.
func BuildKartu(now time.Time) ([]domain.Intervensi, []domain.IntervensiPJ) {
	now = now.UTC()
	pembuat := pembuatDemo
	final := map[string]bool{}
	for _, t := range tahapSeeds {
		final[t.kode] = false
	}
	for _, siklus := range []string{domain.SiklusNormal, domain.SiklusTekanan, domain.SiklusSurplus} {
		var last string
		for _, t := range tahapSeeds {
			if t.siklus == siklus {
				last = t.kode
			}
		}
		final[last] = true
	}

	urutanTahap := map[string]int{} // kode tahap -> posisi (mulai 1) dalam siklusnya
	cacah := map[string]int{}
	for _, t := range tahapSeeds {
		cacah[t.siklus]++
		urutanTahap[t.kode] = cacah[t.siklus]
	}

	var kartu []domain.Intervensi
	var pj []domain.IntervensiPJ
	posisi := map[string]int{}
	for i, s := range kartuSeeds {
		kode := kodeKartu(i)
		k := domain.Intervensi{
			ID: idx.UUIDStabil("intervensi", kode), Kode: &kode, Siklus: s.siklus, Tahap: s.tahap, Urutan: posisi[s.tahap], Judul: s.judul,
			KomoditasKode: s.komoditas, KabupatenKode: s.kabupaten, Prioritas: s.prioritas,
			Tenggat: now.Add(time.Duration(s.tenggat) * day), RekomendasiAI: s.ai,
			ChecklistSelesai: s.selesai, ChecklistTotal: s.total, DibuatOlehID: &pembuat,
			DibuatPada: now.Add(-10*day + time.Duration(i)*time.Hour),
		}
		posisi[s.tahap]++
		k.TahapSejak = k.DibuatPada.Add(time.Duration(urutanTahap[s.tahap]-1) * selangTahap)
		if final[s.tahap] {
			selesai := now.Add(-day)
			k.SelesaiPada = &selesai
		}
		kartu = append(kartu, k)

		pj = append(pj, domain.IntervensiPJ{IntervensiID: k.ID, PenggunaID: timDemo[i%len(timDemo)].id})
		if i%2 == 1 {
			pj = append(pj, domain.IntervensiPJ{IntervensiID: k.ID, PenggunaID: timDemo[(i+3)%len(timDemo)].id})
		}
	}
	return kartu, pj
}

// selangTahap adalah jeda antar pemindahan tahap pada riwayat demo.
const selangTahap = 6 * time.Hour

// BuildRiwayat menyusun riwayat kartu demo: dibuat di tahap pertama siklusnya, lalu
// dipindahkan satu per satu sampai tahap sekarang oleh penanggung jawab pertamanya.
func BuildRiwayat(kartu []domain.Intervensi, pj []domain.IntervensiPJ) []domain.IntervensiRiwayat {
	urutan := map[string][]string{} // siklus -> kode tahap berurutan
	for _, t := range tahapSeeds {
		urutan[t.siklus] = append(urutan[t.siklus], t.kode)
	}
	pertama := map[string]string{} // kartu -> penanggung jawab pertama
	for _, p := range pj {
		if _, ok := pertama[p.IntervensiID]; !ok {
			pertama[p.IntervensiID] = p.PenggunaID
		}
	}

	var out []domain.IntervensiRiwayat
	for _, k := range kartu {
		tahapList := urutan[k.Siklus]
		awal := tahapList[0]
		out = append(out, domain.IntervensiRiwayat{IntervensiID: k.ID, Aksi: domain.AksiBuat, Ke: &awal, Oleh: k.DibuatOlehID, Waktu: k.DibuatPada})
		pelaku := pertama[k.ID]
		for i := 1; i < len(tahapList) && tahapList[i-1] != k.Tahap; i++ {
			dari, ke := tahapList[i-1], tahapList[i]
			out = append(out, domain.IntervensiRiwayat{
				IntervensiID: k.ID, Aksi: domain.AksiPindahTahap, Dari: &dari, Ke: &ke, Oleh: &pelaku,
				Waktu: k.DibuatPada.Add(time.Duration(i) * selangTahap),
			})
		}
	}
	return out
}
