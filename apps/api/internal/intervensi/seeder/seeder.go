// Package seeder mengisi definisi tahap intervensi (referensi, aman di produksi)
// dan kartu kanban contoh (demo, hanya pengembangan). Komoditas dan kabupaten
// berasal dari master.reference; pengguna dari auth.demo; alert dari ews.demo.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Tahap int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Kartu, PenanggungJawab int
}

const day = 24 * time.Hour

// SeedReference meng-upsert definisi tahap tiga siklus. Idempoten.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	tahap := BuildTahap()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "siklus"}, {Name: "kode_tahap"}},
		DoUpdates: clause.AssignmentColumns([]string{"urutan", "label", "petunjuk", "izin_dibutuhkan", "sla_hari"}),
	}).Create(&tahap).Error
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed tahap intervensi: %w", err)
	}
	return ReferenceSummary{Tahap: len(tahap)}, nil
}

// SeedDemo meng-upsert kartu contoh dan mengembalikan penanggung jawabnya ke
// keadaan awal; tenggat relatif terhadap now. Hanya untuk demo.
//
// Prasyarat: SeedReference, master.reference, auth.demo, dan ews.demo sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	kartu, pj := BuildKartu(now)
	ids := make([]string, len(kartu))
	for i, k := range kartu {
		ids[i] = k.ID
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"siklus", "tahap", "urutan", "judul", "deskripsi", "komoditas_kode", "kabupaten_kode", "prioritas",
				"tenggat", "rekomendasi_ai", "checklist_selesai", "checklist_total", "ews_alert_id", "dibuat_oleh_id",
				"dibuat_pada", "selesai_pada",
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
		return nil
	})
	if err != nil {
		return DemoSummary{}, err
	}
	return DemoSummary{Kartu: len(kartu), PenanggungJawab: len(pj)}, nil
}

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

	var kartu []domain.Intervensi
	var pj []domain.IntervensiPJ
	posisi := map[string]int{}
	for i, s := range kartuSeeds {
		k := domain.Intervensi{
			ID: fmt.Sprintf("INT-%d", 301+i), Siklus: s.siklus, Tahap: s.tahap, Urutan: posisi[s.tahap], Judul: s.judul,
			KomoditasKode: s.komoditas, KabupatenKode: s.kabupaten, Prioritas: s.prioritas,
			Tenggat: now.Add(time.Duration(s.tenggat) * day), RekomendasiAI: s.ai,
			ChecklistSelesai: s.selesai, ChecklistTotal: s.total, DibuatOlehID: pembuatDemo,
			DibuatPada: now.Add(-10*day + time.Duration(i)*time.Hour),
		}
		posisi[s.tahap]++
		if s.alert != "" {
			alert := s.alert
			k.EwsAlertID = &alert
		}
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
