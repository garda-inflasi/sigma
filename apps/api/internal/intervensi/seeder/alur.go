package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	labelDewan = "Persetujuan Dewan Pengarah"
	labelDJPb  = "Persetujuan Kanwil DJPb"
)

// alurSeed adalah satu alur persetujuan awal beserta langkahnya (label dan izin).
type alurSeed struct {
	kode, nama, jenis, siklus, tahap string
	langkah                          []langkahSeed
}

type langkahSeed struct{ label, izin string }

// alurSeeds mengikuti tahap persetujuan pada tahapSeeds: tiap tahap yang izinnya bukan
// intervensi.kelola punya alur pemindahan, ditambah satu alur eskalasi. Dukungan fiskal
// melewati Dewan Pengarah lebih dulu, lalu Kanwil DJPb.
var alurSeeds = []alurSeed{
	{"pindah-tekanan-petakan", "Petakan suplai sentra surplus", domain.JenisPindahTahap, domain.SiklusTekanan, "t-petakan",
		[]langkahSeed{{labelDewan, izinSetuju}}},
	{"pindah-tekanan-fiskal", "Dukungan instrumen belanja", domain.JenisPindahTahap, domain.SiklusTekanan, "t-fiskal",
		[]langkahSeed{{labelDewan, izinSetuju}, {labelDJPb, izinSetujuFiskal}}},
	{"pindah-tekanan-selesai", "Penutupan intervensi tekanan harga", domain.JenisPindahTahap, domain.SiklusTekanan, "t-selesai",
		[]langkahSeed{{labelDewan, izinSetuju}}},
	{"pindah-surplus-selesai", "Penutupan intervensi surplus", domain.JenisPindahTahap, domain.SiklusSurplus, "s-selesai",
		[]langkahSeed{{labelDewan, izinSetuju}}},
	{"eskalasi", "Telaah eskalasi kartu", domain.JenisEskalasi, "", "",
		[]langkahSeed{{"Telaah Dewan Pengarah", izinSetuju}}},
}

// BuildAlur menyusun baris alur_persetujuan dan alur_persetujuan_langkah.
func BuildAlur() ([]domain.AlurPersetujuan, []domain.AlurLangkah) {
	var alur []domain.AlurPersetujuan
	var langkah []domain.AlurLangkah
	for _, a := range alurSeeds {
		row := domain.AlurPersetujuan{Kode: a.kode, Nama: a.nama, Jenis: a.jenis, Aktif: true}
		if a.siklus != "" {
			siklus, tahap := a.siklus, a.tahap
			row.Siklus, row.TahapTujuan = &siklus, &tahap
		}
		alur = append(alur, row)
		for i, l := range a.langkah {
			langkah = append(langkah, domain.AlurLangkah{AlurKode: a.kode, Urutan: i + 1, Label: l.label, IzinDibutuhkan: l.izin})
		}
	}
	return alur, langkah
}

// seedAlur menambahkan alur yang belum ada. Alur yang sudah ada tidak disentuh, supaya
// perubahan lewat basis data (langkah, status aktif) bertahan di antara deploy.
func seedAlur(ctx context.Context, db *gorm.DB) (alur, langkah int, err error) {
	a, l := BuildAlur()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
			return fmt.Errorf("seed alur persetujuan: %w", err)
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&l).Error; err != nil {
			return fmt.Errorf("seed langkah alur: %w", err)
		}
		return nil
	})
	return len(a), len(l), err
}

// permintaanDemo adalah satu permintaan persetujuan terbuka pada kartu demo: INT-304 (menunggu
// di Usulan intervensi logistik kilat) diajukan pindah ke Dukungan instrumen belanja.
const (
	kartuPermintaanDemo   = "INT-304"
	tahapPermintaanDemo   = "t-fiskal"
	pengajuPermintaanDemo = "manajer_program"
)

// seedPermintaanDemo membangun ulang permintaan persetujuan demo (id baris dari sequence).
func seedPermintaanDemo(tx *gorm.DB, kartuIDs []string, now time.Time) (int, error) {
	if err := tx.Where("intervensi_id IN ?", kartuIDs).Delete(&domain.IntervensiPersetujuan{}).Error; err != nil {
		return 0, fmt.Errorf("reset permintaan persetujuan: %w", err)
	}
	var id int64
	err := tx.Raw(`
		INSERT INTO intervensi_persetujuan (intervensi_id, alur_kode, jenis, tahap_tujuan, diajukan_oleh, diajukan_pada, catatan)
		VALUES (?, 'pindah-tekanan-fiskal', 'PINDAH_TAHAP', ?, ?, ?, 'Rute dan volume sudah siap; mohon dukungan ongkos angkut')
		RETURNING id`, kartuPermintaanDemo, tahapPermintaanDemo, pengajuPermintaanDemo, now.Add(-3*time.Hour)).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("seed permintaan persetujuan: %w", err)
	}
	// Rantai disalin dari definisi alur seperti pada permintaan nyata.
	err = tx.Exec(`
		INSERT INTO intervensi_persetujuan_langkah (persetujuan_id, urutan, label, izin_dibutuhkan)
		SELECT ?, urutan, label, izin_dibutuhkan FROM alur_persetujuan_langkah WHERE alur_kode = 'pindah-tekanan-fiskal'`, id).Error
	if err != nil {
		return 0, fmt.Errorf("seed langkah permintaan: %w", err)
	}
	return 1, nil
}
