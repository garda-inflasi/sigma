/** Penjelasan istilah dalam bahasa sehari-hari, dipakai sebagai tooltip di seluruh konsol. */
export const GLOSARIUM = {
  normal: "Harga stabil, tidak perlu tindakan.",
  waspada: "Harga mulai naik. Pantau dan siapkan langkah antisipasi.",
  defisit:
    "Pasokan lebih kecil dari kebutuhan sehingga harga tertekan naik. Perlu tindakan segera.",
  surplus:
    "Pasokan melebihi kebutuhan. Wilayah ini bisa menjadi sumber distribusi.",
  siaga:
    "Kenaikan harga sudah melewati ambang. Tindakan perlu disiapkan sekarang.",
  kritis:
    "Kenaikan harga sangat tinggi. Butuh keputusan dan tindakan hari ini.",
  delta:
    "Δ% adalah persentase perubahan harga dibanding periode pembanding. Tanda + berarti harga naik.",
  deviasi:
    "Selisih harga entri terhadap median tujuh hari terakhir di pasar yang sama.",
  inflasi:
    "Kenaikan harga umum dibanding periode yang sama tahun lalu (year-on-year).",
  neraca:
    "Selisih pasokan dan kebutuhan. Positif berarti surplus, negatif berarti defisit.",
  kpi: "Indikator kinerja program, dibandingkan dengan target yang ditetapkan.",
  mape: "Rata-rata kesalahan prediksi dalam persen. Semakin kecil semakin akurat.",
  shelfLife: "Perkiraan sisa masa simpan komoditas sebelum mutunya turun.",
  foodLoss:
    "Bagian pangan yang rusak atau terbuang sebelum sampai ke konsumen.",
  mfa: "Verifikasi dua langkah: selain kata sandi, pengguna memasukkan kode dari aplikasi autentikator.",
  impersonasi:
    "Melihat sistem sebagai peran atau akun lain untuk pengujian. Setiap sesi tercatat di audit trail.",
  siklusTraining:
    "Satu putaran pelatihan model per jam. Semua angka hasil training di halaman ini berasal dari siklus yang dipilih.",
  indeksPangan:
    "Indeks dari harga pangan harian PIHPS, dipakai karena inflasi resmi BPS belum tersedia. Ini bukan angka inflasi resmi.",
  kelayakanModel:
    "Model klasifikasi dianggap layak bila ROC-AUC pada data uji minimal 0,60 dan lebih akurat dari tebakan dasar. Di bawah itu hasilnya belum dapat diandalkan.",
  persistensi:
    "Baseline yang menganggap harga ke depan sama dengan harga terakhir (atau pola musimannya). Dipakai bila tidak ada model yang lebih akurat.",
  rentangKuantil:
    "P10 dan P90: 80% kemungkinan harga berada di antara keduanya. P50 adalah nilai tengah proyeksi.",
  rmse: "Rata-rata besar kesalahan prediksi dalam rupiah. Semakin kecil semakin akurat.",
  kontribusiFaktor:
    "Seberapa besar tiap kelompok faktor menggeser proyeksi. Satuannya tertulis di samping angka.",
} as const;

export type IstilahGlosarium = keyof typeof GLOSARIUM;
