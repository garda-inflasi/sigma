# Sumber batas wilayah Aceh

`aceh-kabupaten.geojson` dan `db/seeds/reference/10_spasial_geometri.sql` memuat batas 23 kabupaten/kota Provinsi Aceh.

- **Data asal:** Badan Pusat Statistik (BPS), World Food Programme, dan OCHA ROAP, tahun 2020.
- **Distribusi:** geoBoundaries gbOpen `IDN` ADM2, commit `9469f09`, berkas `_simplified`.
- **Lisensi:** Creative Commons Attribution 3.0 IGO (CC BY 3.0 IGO). Atribusi wajib tetap tampil di peta.
- **Pengolahan:** ekstrak 23 unit Aceh, kode BPS dipasangkan lewat nama, koordinat dibulatkan 5 desimal, titik berurutan yang sama dihapus.
- **Urutan koordinat:** GeoJSON memakai `[bujur, lintang]`. Leaflet memakai `[lintang, bujur]`, lihat `src/lib/peta/aceh.ts`.

Batas ini versi disederhanakan untuk tampilan dan pencarian wilayah, bukan dokumen hukum batas administratif.
`src/lib/peta/aceh.test.ts` memastikan pusat kabupaten/kota, pasar, dan cold storage berada di wilayah yang benar.
