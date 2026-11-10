package postgres

import (
	"context"
	"database/sql"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"gorm.io/gorm"
)

// kondisiAsalAtauTujuan menerjemahkan Cakupan dan filter kabupaten permintaan menjadi kondisi SQL untuk baris yang
// punya DUA ujung wilayah (pergerakan stok: fasilitas asal dan tujuan; rute: kabupaten asal dan tujuan). Baris masuk
// cakupan bila SALAH SATU ujungnya di kabupaten efektif: Koordinator di kabupaten tujuan melihat kiriman yang masuk ke
// kabupatennya, dan yang di kabupaten asal melihat kiriman yang keluar, walau ujung satunya di luar cakupan. Ujung
// yang NULL (MASUK tidak punya asal, KELUAR tidak punya tujuan) tidak pernah cocok. Irisannya dihitung Cakupan.Irisi
// seperti kondisiKabupaten: cakupan tertutup dan filter di luar cakupan menjadi FALSE (gagal-tutup).
func kondisiAsalAtauTujuan(kolomAsal, kolomTujuan string, c domain.Cakupan, filter string) (string, []any) {
	kabupaten, ada := c.Irisi(filter)
	switch {
	case !ada:
		return "FALSE", nil
	case kabupaten == "":
		return "TRUE", nil
	}
	return "(" + kolomAsal + " = ? OR " + kolomTujuan + " = ?)", []any{kabupaten, kabupaten}
}

// dalamSnapshot menjalankan fn dalam transaksi hanya-baca REPEATABLE READ: semua query di dalamnya melihat keadaan
// basis data yang sama, sehingga ringkasan dan rinciannya (mis. total stok dan stok per komoditas) tidak pernah
// saling bertentangan karena ada transaksi stok yang masuk di sela-selanya.
func (r *Repository) dalamSnapshot(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}
