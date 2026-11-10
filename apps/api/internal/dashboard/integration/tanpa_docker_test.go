package integration

import (
	"fmt"
	"testing"
)

// Tanpa Docker (`go test -short`, dipakai make test/api/short) setiap tes yang butuh basis data harus di-SKIP, bukan
// gagal. Penyedia suite bersama memakai sync.Once, dan t.Skip di dalam Once menghentikan goroutine pemanggil dengan
// runtime.Goexit: Once menganggap fungsinya selesai, suite-nya tetap nil, dan pemanggil berikutnya gagal pada
// require.NotNil ("suite bersama gagal dibuat") padahal seharusnya ikut di-skip. perluDocker mengecek di depan.
//
// Tes ini memanggil tiap penyedia berulang kali dengan pg dikosongkan, seperti pada -short. Setiap pemanggilan harus
// berakhir sebagai SKIP: gagal berarti penyedia itu lagi-lagi menghabiskan Once-nya sebelum Docker dicek, dan lolos
// begitu saja (tidak di-skip) berarti ia mencoba memakai basis data yang tidak ada. Tes ini berjalan sama di mode biasa
// dan -short, dan tidak menyentuh suite bersama yang sebenarnya karena penyedia berhenti sebelum sync.Once.
func TestTanpaDocker_SemuaPenyediaSuiteMelewatiBukanGagal(t *testing.T) {
	asli := pg
	pg = nil
	t.Cleanup(func() { pg = asli })

	penyedia := []struct {
		nama  string
		ambil func(t *testing.T)
	}{
		{"sharedSuite", func(t *testing.T) { sharedSuite(t) }},
		{"ingestionSuite", func(t *testing.T) { ingestionSuite(t) }},
		{"kontrakSuite", func(t *testing.T) { kontrakSuite(t) }},
		{"newSuite", func(t *testing.T) { newSuite(t, "tanpa_docker", true) }},
		{"terpisah", func(t *testing.T) { terpisah(t, "tanpa_docker", false) }},
		{"seedDB", func(t *testing.T) { seedDB(t, "tanpa_docker") }},
	}
	for _, p := range penyedia {
		// Pemanggilan kedua dan ketiga adalah yang dulu gagal: Once sudah "selesai" dengan suite nil.
		for ke := 1; ke <= 3; ke++ {
			t.Run(fmt.Sprintf("%s pemanggilan ke-%d", p.nama, ke), func(t *testing.T) {
				p.ambil(t)
				t.Errorf("%s tidak di-skip padahal container PostgreSQL tidak ada", p.nama)
			})
		}
	}
}
