package geoip

// Berantai mencoba resolver berurutan dan memakai jawaban tak-kosong pertama. Aman dipanggil bersamaan bila
// semua resolvernya demikian.
type Berantai []Resolver

// Lokasi memenuhi Resolver.
func (b Berantai) Lokasi(ip string) string {
	for _, r := range b {
		if r == nil {
			continue
		}
		if lokasi := r.Lokasi(ip); lokasi != "" {
			return lokasi
		}
	}
	return ""
}

// DariBerkas membuat resolver yang dipakai API: klasifikasi jaringan lokal selalu aktif dan didahulukan (tidak
// perlu membuka basis data untuk 10.0.0.1), lalu basis data MMDB di path bila diisi. Path kosong berarti tanpa
// basis data: alamat publik tidak diberi lokasi. Galat bila path terisi tetapi basis datanya tidak dapat dibuka;
// pemanggil menghentikan start, supaya salah konfigurasi tidak berjalan diam-diam tanpa lokasi.
func DariBerkas(path string) (Resolver, error) {
	if path == "" {
		return Berantai{Lokal{}}, nil
	}
	mmdb, err := BukaMMDB(path)
	if err != nil {
		return nil, err
	}
	return Berantai{Lokal{}, mmdb}, nil
}
