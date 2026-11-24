/**
 * Fixture respons nyata lampiran `ml` API Go (docs/PIPELINE-DATA-ML.md §5), dipangkas agar ringkas.
 *
 * Asal: API Go (`apps/api`, `cmd/api`) dengan `MONGO_URI` ke MongoDB dev (baca saja) di atas basis data uji hasil
 * migrasi + seed reference + demo, login superadmin, diambil 24 Nov 2026 12.05 WIB. Tiga siklus nyata:
 *   - C20261124T05Z: format baru, klasifikasi `layak: false` (TIDAK_LAYAK) dengan alasan, kelas/skor/siklus kosong;
 *   - C20261124T04Z: format lama tanpa `layak` (kelayakan TIDAK_DIKETAHUI), XAI tanpa `satuan_kontribusi`;
 *   - C20261124T03Z: SEBAGIAN tanpa forecast (TIDAK_ADA_DATA).
 * Daftar dipangkas (riwayat 8 titik terakhir, 4 forecast, 3 sinyal, satu sumber per kategori kesegaran); nilai tidak
 * diubah. Tidak memuat token, email, atau data pribadi.
 */
import type { DaftarSiklusMlMentah, LampiranMlMentah } from "../../ml";


export const SIKLUS_NYATA: DaftarSiklusMlMentah = {
  "tersedia": true,
  "items": [
    {
      "cycle_id": "C20261124T05Z",
      "cycle_timestamp": "2026-11-24T05:00:00Z",
      "data_cutoff": "2026-11-24T05:00:00Z",
      "status": "BERHASIL",
      "final": true,
      "duration_ms": 6569,
      "pesan": "tidak ada data baru sejak siklus C20261124T04Z; klasifikasi INDEKS_PANGAN belum layak: ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249",
      "versi_kode": "0.1.0+pipeline.1.0.0",
      "jumlah": {
        "datasafe_ews_signals": 13,
        "datasafe_forecasts": 30,
        "datasafe_inflasi_signals": 3,
        "datasafe_macro_indicators": 0,
        "datasafe_xai_explanations": 30
      },
      "target_inflasi": "INDEKS_PANGAN",
      "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
      "model_metrics": {
        "klasifikasi": {
          "model": "lightgbm",
          "f1": 0.576271186440678,
          "roc_auc": 0.4094746716697936,
          "brier": 0.25107863899268434,
          "brier_baseline": 0.2490694492065993,
          "n_uji": 93,
          "kalibrasi": "sigmoid",
          "layak": false,
          "kelayakan": "TIDAK_LAYAK",
          "alasan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249"
        },
        "regresi": {
          "pemenang": {
            "holt_winters": 0,
            "lightgbm_quantile": 0,
            "naive": 22,
            "sarimax": 0,
            "seasonal_naive": 8
          },
          "mape_median": 0.9698231810367397,
          "rmse_median": 363.4839432971472,
          "mape_baseline_median": 0.9698231810367397,
          "n_seri": 30,
          "mengalahkan_baseline": 0,
          "n_dievaluasi_holdout": 30,
          "n_dilewati": 0,
          "n_gagal": 0,
          "model_global": "lightgbm_quantile",
          "tingkat": "FIT_ULANG"
        }
      }
    },
    {
      "cycle_id": "C20261124T04Z",
      "cycle_timestamp": "2026-11-24T04:00:00Z",
      "data_cutoff": "2026-11-24T04:00:00Z",
      "status": "BERHASIL",
      "final": true,
      "duration_ms": 26587,
      "pesan": "",
      "versi_kode": "0.1.0+pipeline.1.0.0",
      "jumlah": {
        "datasafe_ews_signals": 14,
        "datasafe_forecasts": 30,
        "datasafe_inflasi_signals": 3,
        "datasafe_macro_indicators": 0,
        "datasafe_xai_explanations": 33
      },
      "target_inflasi": "INDEKS_PANGAN",
      "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
      "model_metrics": {
        "klasifikasi": {
          "model": "lightgbm",
          "f1": 0.56,
          "roc_auc": 0.5379867046533713,
          "brier": 0.2476140330793081,
          "brier_baseline": null,
          "n_uji": 93,
          "kalibrasi": "sigmoid",
          "layak": null,
          "kelayakan": "TIDAK_DIKETAHUI",
          "alasan": null
        },
        "regresi": {
          "pemenang": {
            "holt_winters": 0,
            "lightgbm_quantile": 0,
            "naive": 22,
            "sarimax": 0,
            "seasonal_naive": 8
          },
          "mape_median": 0.9698231810367397,
          "rmse_median": 363.4839432971472,
          "mape_baseline_median": 0.9698231810367397,
          "n_seri": 30,
          "mengalahkan_baseline": 0,
          "n_dievaluasi_holdout": 30,
          "n_dilewati": 0,
          "n_gagal": 0,
          "model_global": "lightgbm_quantile",
          "tingkat": "FIT_ULANG"
        }
      }
    },
    {
      "cycle_id": "C20261124T03Z",
      "cycle_timestamp": "2026-11-24T03:00:00Z",
      "data_cutoff": "2026-11-24T03:00:00Z",
      "status": "SEBAGIAN",
      "final": true,
      "duration_ms": 337,
      "pesan": "tidak ada titik harga asli yang dapat dipakai; klasifikasi dilewati: tidak ada data harga",
      "versi_kode": "0.1.0+pipeline.1.0.0",
      "jumlah": {
        "datasafe_ews_signals": 1,
        "datasafe_forecasts": 0,
        "datasafe_inflasi_signals": 0,
        "datasafe_macro_indicators": 0,
        "datasafe_xai_explanations": 0
      },
      "target_inflasi": null,
      "label_target": null,
      "model_metrics": {
        "klasifikasi": {
          "model": null,
          "f1": null,
          "roc_auc": null,
          "brier": null,
          "brier_baseline": null,
          "n_uji": 0,
          "kalibrasi": null,
          "layak": null,
          "kelayakan": "TIDAK_DIKETAHUI",
          "alasan": "panel harga kosong"
        },
        "regresi": {
          "pemenang": {
            "holt_winters": 0,
            "lightgbm_quantile": 0,
            "naive": 0,
            "sarimax": 0,
            "seasonal_naive": 0
          },
          "mape_median": null,
          "rmse_median": null,
          "mape_baseline_median": null,
          "n_seri": 0,
          "mengalahkan_baseline": 0,
          "n_dievaluasi_holdout": 0,
          "n_dilewati": 0,
          "n_gagal": 0,
          "model_global": "lightgbm_quantile",
          "tingkat": "PENUH"
        }
      }
    }
  ],
  "total": 3,
  "page": 1,
  "limit": 24
};

export const EKSEKUTIF_C05: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "data_cutoff": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "inflasi": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "kelayakan": "TIDAK_LAYAK",
    "alasan_kelayakan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249",
    "target": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
    "bukan_inflasi_resmi": true,
    "wilayah": [
      {
        "wilayah_kode": "1107",
        "tingkat": "KABUPATEN",
        "periode": "2026-12-24",
        "kelas": null,
        "layak": false,
        "kelayakan": "TIDAK_LAYAK",
        "alasan_kelayakan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249",
        "probabilitas": 0.449812,
        "skor_tekanan": null,
        "siklus": "",
        "target": "INDEKS_PANGAN",
        "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
        "bukan_inflasi_resmi": true,
        "proyeksi": {
          "yoy": null,
          "mtm": 0,
          "ytd": null
        },
        "aktual": {
          "yoy": 5.1643,
          "mtm": 0.7965,
          "ytd": 4.9163,
          "periode": "2026-11-24"
        },
        "penyumbang": [
          {
            "komoditas_kode": "CBM-KRT",
            "nama": "Cabai Merah Keriting",
            "andil": 1.7669
          },
          {
            "komoditas_kode": "DAG-AYM",
            "nama": "Daging Ayam Ras Segar",
            "andil": -1.6735
          },
          {
            "komoditas_kode": "BWM",
            "nama": "Bawang Merah Ukuran Sedang",
            "andil": 1.6667
          },
          {
            "komoditas_kode": "BWP",
            "nama": "Bawang Putih Ukuran Sedang",
            "andil": -0.3205
          },
          {
            "komoditas_kode": "TLR",
            "nama": "Telur Ayam Ras Segar",
            "andil": -0.2773
          }
        ],
        "kalibrasi": {
          "metode": "sigmoid",
          "brier": 0.25107863899268434
        },
        "penjelasan": null
      },
      {
        "wilayah_kode": "1171",
        "tingkat": "KABUPATEN",
        "periode": "2026-12-24",
        "kelas": null,
        "layak": false,
        "kelayakan": "TIDAK_LAYAK",
        "alasan_kelayakan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249",
        "probabilitas": 0.485315,
        "skor_tekanan": null,
        "siklus": "",
        "target": "INDEKS_PANGAN",
        "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
        "bukan_inflasi_resmi": true,
        "proyeksi": {
          "yoy": null,
          "mtm": -0.5378,
          "ytd": null
        },
        "aktual": {
          "yoy": 1.4634,
          "mtm": 2.0921,
          "ytd": -1.5675,
          "periode": "2026-11-24"
        },
        "penyumbang": [
          {
            "komoditas_kode": "CBM-KRT",
            "nama": "Cabai Merah Keriting",
            "andil": 1.5686
          },
          {
            "komoditas_kode": "BWM",
            "nama": "Bawang Merah Ukuran Sedang",
            "andil": 0.8696
          },
          {
            "komoditas_kode": "DAG-AYM",
            "nama": "Daging Ayam Ras Segar",
            "andil": -0.3846
          },
          {
            "komoditas_kode": "MYK",
            "nama": "Minyak Goreng Curah",
            "andil": 0.2381
          },
          {
            "komoditas_kode": "BWP",
            "nama": "Bawang Putih Ukuran Sedang",
            "andil": 0.2143
          }
        ],
        "kalibrasi": {
          "metode": "sigmoid",
          "brier": 0.25107863899268434
        },
        "penjelasan": null
      },
      {
        "wilayah_kode": "1174",
        "tingkat": "KABUPATEN",
        "periode": "2026-12-24",
        "kelas": null,
        "layak": false,
        "kelayakan": "TIDAK_LAYAK",
        "alasan_kelayakan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249",
        "probabilitas": 0.49869,
        "skor_tekanan": null,
        "siklus": "",
        "target": "INDEKS_PANGAN",
        "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
        "bukan_inflasi_resmi": true,
        "proyeksi": {
          "yoy": null,
          "mtm": 0.1455,
          "ytd": null
        },
        "aktual": {
          "yoy": 3.3762,
          "mtm": 1.8884,
          "ytd": 5.6235,
          "periode": "2026-11-24"
        },
        "penyumbang": [
          {
            "komoditas_kode": "BWM",
            "nama": "Bawang Merah Ukuran Sedang",
            "andil": 1.9643
          },
          {
            "komoditas_kode": "CBM-KRT",
            "nama": "Cabai Merah Keriting",
            "andil": 1.1742
          },
          {
            "komoditas_kode": "DAG-AYM",
            "nama": "Daging Ayam Ras Segar",
            "andil": -0.4965
          },
          {
            "komoditas_kode": "TLR",
            "nama": "Telur Ayam Ras Segar",
            "andil": -0.2456
          },
          {
            "komoditas_kode": "DAG-SPI",
            "nama": "Daging Sapi Kualitas 1",
            "andil": -0.1449
          }
        ],
        "kalibrasi": {
          "metode": "sigmoid",
          "brier": 0.25107863899268434
        },
        "penjelasan": null
      }
    ]
  },
  "model": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
    "klasifikasi": {
      "model": "lightgbm",
      "f1": 0.576271186440678,
      "roc_auc": 0.4094746716697936,
      "brier": 0.25107863899268434,
      "brier_baseline": 0.2490694492065993,
      "n_uji": 93,
      "kalibrasi": "sigmoid",
      "layak": false,
      "kelayakan": "TIDAK_LAYAK",
      "alasan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249"
    },
    "regresi": {
      "pemenang": {
        "holt_winters": 0,
        "lightgbm_quantile": 0,
        "naive": 22,
        "sarimax": 0,
        "seasonal_naive": 8
      },
      "mape_median": 0.9698231810367397,
      "rmse_median": 363.4839432971472,
      "mape_baseline_median": 0.9698231810367397,
      "n_seri": 30,
      "mengalahkan_baseline": 0,
      "n_dievaluasi_holdout": 30,
      "n_dilewati": 0,
      "n_gagal": 0,
      "model_global": "lightgbm_quantile",
      "tingkat": "FIT_ULANG"
    }
  },
  "ews": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "ringkasan": {
      "aktif": 13,
      "kritis": 0,
      "siaga": 1,
      "waspada": 0,
      "info": 12
    },
    "sinyal": [
      {
        "sinyal_kunci": "ANOMALI_HARGA:DAG-AYM:1107:TURUN",
        "severity": "SIAGA",
        "jenis": "ANOMALI_HARGA",
        "komoditas_kode": "DAG-AYM",
        "kabupaten_kode": "1107",
        "nilai": -17.297,
        "ambang": 10,
        "metode": "zscore_musiman",
        "judul": "Anomali harga Daging Ayam Ras Segar (turun)",
        "pesan": "Harga Daging Ayam Ras Segar di wilayah 1107 pada 2026-11-23 menyimpang -17.3% dari pola musimannya (z=-3.6).",
        "aktif": true,
        "menjadi_alert": true,
        "alert": null
      },
      {
        "sinyal_kunci": "SUMBER_DATA:BAPANAS:panel_harga",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BAPANAS dilewati: panel_harga",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      },
      {
        "sinyal_kunci": "SUMBER_DATA:BMKG:peringatan_cuaca",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BMKG dilewati: peringatan_cuaca",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: robots.txt www.bmkg.go.id melarang /alerts dan /api (diperiksa 2026-11-24); tindakan: minta akses resmi data peringatan cuaca ke BMKG",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      }
    ],
    "terpotong": true
  },
  "kesegaran": {
    "tersedia": true,
    "sumber_data": "DATAUNSEEN",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "dihitung_pada": "2026-11-24T05:07:10Z",
    "siklus": {
      "cycle_id": "C20261124T05Z",
      "cycle_timestamp": "2026-11-24T05:00:00Z",
      "data_cutoff": "2026-11-24T05:00:00Z",
      "status": "BERHASIL",
      "target_inflasi": "INDEKS_PANGAN",
      "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
    },
    "ringkasan": {
      "segar": 1,
      "basi": 1,
      "gagal": 0,
      "dilewati": 12,
      "belum_ada_run_terjadwal": 0
    },
    "sumber": [
      {
        "sumber": "BAPANAS",
        "dataset": "panel_harga",
        "kategori": "DILEWATI",
        "status": "ERROR",
        "pesan": "DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "last_available_date": null,
        "umur_hari": null,
        "run_terakhir": {
          "run_id": "2026-11-24:BAPANAS:panel_harga:MANUAL",
          "crawler": "garda-crawler/0.1.0",
          "crawl_date": "2026-11-24",
          "mulai": "2026-11-24T03:50:04Z",
          "selesai": "2026-11-24T03:50:04Z",
          "status": "ERROR",
          "jumlah_baru": 0,
          "jumlah_diteruskan": 0,
          "last_available_date": null,
          "pesan": "DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
          "pemicu": "MANUAL",
          "dilewati": true
        },
        "menurut_siklus": {
          "status": "ERROR",
          "last_available_date": null,
          "crawled_at": "2026-11-24T03:50:04Z",
          "umur_hari": null,
          "pesan": "DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas"
        }
      },
      {
        "sumber": "BMKG",
        "dataset": "prakiraan_cuaca_adm4",
        "kategori": "SEGAR",
        "status": "FRESH",
        "pesan": "permintaan 23, gagal 0 | ditolak 23 (hari_tidak_lengkap=23)",
        "last_available_date": "2026-11-26",
        "umur_hari": 0,
        "run_terakhir": {
          "run_id": "2026-11-24:BMKG:prakiraan_cuaca_adm4:MANUAL",
          "crawler": "garda-crawler/0.1.0",
          "crawl_date": "2026-11-24",
          "mulai": "2026-11-24T03:49:17Z",
          "selesai": "2026-11-24T03:50:03Z",
          "status": "FRESH",
          "jumlah_baru": 138,
          "jumlah_diteruskan": 0,
          "last_available_date": "2026-11-26",
          "pesan": "permintaan 23, gagal 0 | ditolak 23 (hari_tidak_lengkap=23)",
          "pemicu": "MANUAL",
          "dilewati": false
        },
        "menurut_siklus": {
          "status": "FRESH",
          "last_available_date": "2026-11-26",
          "crawled_at": "2026-11-24T03:50:03Z",
          "umur_hari": -2,
          "pesan": "permintaan 23, gagal 0 | ditolak 23 (hari_tidak_lengkap=23)"
        }
      },
      {
        "sumber": "PIHPS",
        "dataset": "harga_pasar_tradisional",
        "kategori": "BASI",
        "status": "STALE_CARRIED_FORWARD",
        "pesan": "permintaan 23, gagal 0",
        "last_available_date": "2026-11-23",
        "umur_hari": 1,
        "run_terakhir": {
          "run_id": "2026-11-24:PIHPS:harga_pasar_tradisional:MANUAL",
          "crawler": "garda-crawler/0.1.0",
          "crawl_date": "2026-11-24",
          "mulai": "2026-11-24T03:48:30Z",
          "selesai": "2026-11-24T03:49:17Z",
          "status": "STALE_CARRIED_FORWARD",
          "jumlah_baru": 0,
          "jumlah_diteruskan": 31,
          "last_available_date": "2026-11-23",
          "pesan": "permintaan 23, gagal 0",
          "pemicu": "MANUAL",
          "dilewati": false
        },
        "menurut_siklus": {
          "status": "STALE_CARRIED_FORWARD",
          "last_available_date": "2026-11-23",
          "crawled_at": "2026-11-24T03:49:17Z",
          "umur_hari": 1,
          "pesan": "permintaan 23, gagal 0"
        }
      }
    ],
    "sinyal_sumber": [
      {
        "sinyal_kunci": "SUMBER_DATA:BAPANAS:panel_harga",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BAPANAS dilewati: panel_harga",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      }
    ]
  },
  "makro": {
    "tersedia": false,
    "alasan": "TIDAK_ADA_DATA",
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "items": []
  }
};

export const EKSEKUTIF_C04: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "data_cutoff": "2026-11-24T04:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "inflasi": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "kelayakan": "TIDAK_DIKETAHUI",
    "alasan_kelayakan": null,
    "target": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
    "bukan_inflasi_resmi": true,
    "wilayah": [
      {
        "wilayah_kode": "1107",
        "tingkat": "KABUPATEN",
        "periode": "2026-12-24",
        "kelas": "TIDAK_INFLASI",
        "layak": null,
        "kelayakan": "TIDAK_DIKETAHUI",
        "alasan_kelayakan": null,
        "probabilitas": 0.485306,
        "skor_tekanan": 47.29,
        "siklus": "NORMAL",
        "target": "INDEKS_PANGAN",
        "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
        "bukan_inflasi_resmi": true,
        "proyeksi": {
          "yoy": null,
          "mtm": 0,
          "ytd": null
        },
        "aktual": {
          "yoy": 5.1643,
          "mtm": 0.7965,
          "ytd": 4.9163,
          "periode": "2026-11-24"
        },
        "penyumbang": [
          {
            "komoditas_kode": "CBM-KRT",
            "nama": "Cabai Merah Keriting",
            "andil": 1.7669
          },
          {
            "komoditas_kode": "DAG-AYM",
            "nama": "Daging Ayam Ras Segar",
            "andil": -1.6735
          },
          {
            "komoditas_kode": "BWM",
            "nama": "Bawang Merah Ukuran Sedang",
            "andil": 1.6667
          },
          {
            "komoditas_kode": "BWP",
            "nama": "Bawang Putih Ukuran Sedang",
            "andil": -0.3205
          },
          {
            "komoditas_kode": "TLR",
            "nama": "Telur Ayam Ras Segar",
            "andil": -0.2773
          }
        ],
        "kalibrasi": {
          "metode": "sigmoid",
          "brier": 0.2476140330793081
        },
        "penjelasan": {
          "metode": "shap",
          "satuan_kontribusi": null,
          "faktor": [
            {
              "kelompok": "KALENDER",
              "label": "Kalender hari besar",
              "kontribusi": 126.3044,
              "arah": "NAIK",
              "fitur": [
                {
                  "nama": "kal_hari_ke_tahun_baru",
                  "kontribusi": 48.8405
                },
                {
                  "nama": "kal_bulan",
                  "kontribusi": 46.6569
                },
                {
                  "nama": "kal_hari_ke_natal",
                  "kontribusi": 14.3112
                }
              ]
            },
            {
              "kelompok": "TREN_HARGA_LAPANGAN",
              "label": "Tren harga dan lapangan",
              "kontribusi": 100.9738,
              "arah": "NAIK",
              "fitur": [
                {
                  "nama": "vol_30",
                  "kontribusi": 64.3346
                },
                {
                  "nama": "lag_30",
                  "kontribusi": 39.6498
                },
                {
                  "nama": "ma_7",
                  "kontribusi": 34.3257
                }
              ]
            }
          ],
          "narasi": "Probabilitas kenaikan indeks harga pangan (bukan inflasi resmi) di wilayah 1107 sebesar 49%, siklus NORMAL. Faktor terbesar: kalender hari besar, tren harga dan lapangan."
        }
      },
      {
        "wilayah_kode": "1171",
        "tingkat": "KABUPATEN",
        "periode": "2026-12-24",
        "kelas": "INFLASI",
        "layak": null,
        "kelayakan": "TIDAK_DIKETAHUI",
        "alasan_kelayakan": null,
        "probabilitas": 0.508389,
        "skor_tekanan": 49.54,
        "siklus": "NORMAL",
        "target": "INDEKS_PANGAN",
        "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
        "bukan_inflasi_resmi": true,
        "proyeksi": {
          "yoy": null,
          "mtm": -0.5378,
          "ytd": null
        },
        "aktual": {
          "yoy": 1.4634,
          "mtm": 2.0921,
          "ytd": -1.5675,
          "periode": "2026-11-24"
        },
        "penyumbang": [
          {
            "komoditas_kode": "CBM-KRT",
            "nama": "Cabai Merah Keriting",
            "andil": 1.5686
          },
          {
            "komoditas_kode": "BWM",
            "nama": "Bawang Merah Ukuran Sedang",
            "andil": 0.8696
          },
          {
            "komoditas_kode": "DAG-AYM",
            "nama": "Daging Ayam Ras Segar",
            "andil": -0.3846
          },
          {
            "komoditas_kode": "MYK",
            "nama": "Minyak Goreng Curah",
            "andil": 0.2381
          },
          {
            "komoditas_kode": "BWP",
            "nama": "Bawang Putih Ukuran Sedang",
            "andil": 0.2143
          }
        ],
        "kalibrasi": {
          "metode": "sigmoid",
          "brier": 0.2476140330793081
        },
        "penjelasan": {
          "metode": "shap",
          "satuan_kontribusi": null,
          "faktor": [
            {
              "kelompok": "KALENDER",
              "label": "Kalender hari besar",
              "kontribusi": 114.8701,
              "arah": "NAIK",
              "fitur": [
                {
                  "nama": "kal_hari_ke_tahun_baru",
                  "kontribusi": 45.8013
                },
                {
                  "nama": "kal_bulan",
                  "kontribusi": 29.0183
                },
                {
                  "nama": "kal_hari_ke_natal",
                  "kontribusi": 20.8045
                }
              ]
            },
            {
              "kelompok": "TREN_HARGA_LAPANGAN",
              "label": "Tren harga dan lapangan",
              "kontribusi": 88.6969,
              "arah": "NAIK",
              "fitur": [
                {
                  "nama": "ma_7",
                  "kontribusi": 38.1654
                },
                {
                  "nama": "ma_30",
                  "kontribusi": -37.8069
                },
                {
                  "nama": "lag_3",
                  "kontribusi": 21.0186
                }
              ]
            }
          ],
          "narasi": "Probabilitas kenaikan indeks harga pangan (bukan inflasi resmi) di wilayah 1171 sebesar 51%, siklus NORMAL. Faktor terbesar: kalender hari besar, tren harga dan lapangan."
        }
      },
      {
        "wilayah_kode": "1174",
        "tingkat": "KABUPATEN",
        "periode": "2026-12-24",
        "kelas": "INFLASI",
        "layak": null,
        "kelayakan": "TIDAK_DIKETAHUI",
        "alasan_kelayakan": null,
        "probabilitas": 0.508699,
        "skor_tekanan": 49.57,
        "siklus": "NORMAL",
        "target": "INDEKS_PANGAN",
        "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
        "bukan_inflasi_resmi": true,
        "proyeksi": {
          "yoy": null,
          "mtm": 0.1455,
          "ytd": null
        },
        "aktual": {
          "yoy": 3.3762,
          "mtm": 1.8884,
          "ytd": 5.6235,
          "periode": "2026-11-24"
        },
        "penyumbang": [
          {
            "komoditas_kode": "BWM",
            "nama": "Bawang Merah Ukuran Sedang",
            "andil": 1.9643
          },
          {
            "komoditas_kode": "CBM-KRT",
            "nama": "Cabai Merah Keriting",
            "andil": 1.1742
          },
          {
            "komoditas_kode": "DAG-AYM",
            "nama": "Daging Ayam Ras Segar",
            "andil": -0.4965
          },
          {
            "komoditas_kode": "TLR",
            "nama": "Telur Ayam Ras Segar",
            "andil": -0.2456
          },
          {
            "komoditas_kode": "DAG-SPI",
            "nama": "Daging Sapi Kualitas 1",
            "andil": -0.1449
          }
        ],
        "kalibrasi": {
          "metode": "sigmoid",
          "brier": 0.2476140330793081
        },
        "penjelasan": {
          "metode": "shap",
          "satuan_kontribusi": null,
          "faktor": [
            {
              "kelompok": "KALENDER",
              "label": "Kalender hari besar",
              "kontribusi": 155.1899,
              "arah": "NAIK",
              "fitur": [
                {
                  "nama": "kal_hari_ke_tahun_baru",
                  "kontribusi": 54.8068
                },
                {
                  "nama": "kal_bulan",
                  "kontribusi": 54.1054
                },
                {
                  "nama": "kal_hari_ke_natal",
                  "kontribusi": 28.5796
                }
              ]
            },
            {
              "kelompok": "TREN_HARGA_LAPANGAN",
              "label": "Tren harga dan lapangan",
              "kontribusi": -42.0301,
              "arah": "TURUN",
              "fitur": [
                {
                  "nama": "luas_kenaikan_30",
                  "kontribusi": -41.1754
                },
                {
                  "nama": "vol_30",
                  "kontribusi": 20.094
                },
                {
                  "nama": "lag_14",
                  "kontribusi": 15.9999
                }
              ]
            }
          ],
          "narasi": "Probabilitas kenaikan indeks harga pangan (bukan inflasi resmi) di wilayah 1174 sebesar 51%, siklus NORMAL. Faktor terbesar: kalender hari besar, tren harga dan lapangan."
        }
      }
    ]
  },
  "model": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
    "klasifikasi": {
      "model": "lightgbm",
      "f1": 0.56,
      "roc_auc": 0.5379867046533713,
      "brier": 0.2476140330793081,
      "brier_baseline": null,
      "n_uji": 93,
      "kalibrasi": "sigmoid",
      "layak": null,
      "kelayakan": "TIDAK_DIKETAHUI",
      "alasan": null
    },
    "regresi": {
      "pemenang": {
        "holt_winters": 0,
        "lightgbm_quantile": 0,
        "naive": 22,
        "sarimax": 0,
        "seasonal_naive": 8
      },
      "mape_median": 0.9698231810367397,
      "rmse_median": 363.4839432971472,
      "mape_baseline_median": 0.9698231810367397,
      "n_seri": 30,
      "mengalahkan_baseline": 0,
      "n_dievaluasi_holdout": 30,
      "n_dilewati": 0,
      "n_gagal": 0,
      "model_global": "lightgbm_quantile",
      "tingkat": "FIT_ULANG"
    }
  },
  "ews": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "ringkasan": {
      "aktif": 13,
      "kritis": 0,
      "siaga": 1,
      "waspada": 0,
      "info": 12
    },
    "sinyal": [
      {
        "sinyal_kunci": "ANOMALI_HARGA:DAG-AYM:1107:TURUN",
        "severity": "SIAGA",
        "jenis": "ANOMALI_HARGA",
        "komoditas_kode": "DAG-AYM",
        "kabupaten_kode": "1107",
        "nilai": -17.297,
        "ambang": 10,
        "metode": "zscore_musiman",
        "judul": "Anomali harga Daging Ayam Ras Segar (turun)",
        "pesan": "Harga Daging Ayam Ras Segar di wilayah 1107 pada 2026-11-23 menyimpang -17.3% dari pola musimannya (z=-3.6).",
        "aktif": true,
        "menjadi_alert": true,
        "alert": null
      },
      {
        "sinyal_kunci": "SUMBER_DATA:BAPANAS:panel_harga",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BAPANAS dilewati: panel_harga",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      },
      {
        "sinyal_kunci": "SUMBER_DATA:BMKG:peringatan_cuaca",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BMKG dilewati: peringatan_cuaca",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: robots.txt www.bmkg.go.id melarang /alerts dan /api (diperiksa 2026-11-24); tindakan: minta akses resmi data peringatan cuaca ke BMKG",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      }
    ],
    "terpotong": true
  },
  "kesegaran": {
    "tersedia": true,
    "sumber_data": "DATAUNSEEN",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "dihitung_pada": "2026-11-24T05:07:10Z",
    "siklus": {
      "cycle_id": "C20261124T05Z",
      "cycle_timestamp": "2026-11-24T05:00:00Z",
      "data_cutoff": "2026-11-24T05:00:00Z",
      "status": "BERHASIL",
      "target_inflasi": "INDEKS_PANGAN",
      "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
    },
    "ringkasan": {
      "segar": 1,
      "basi": 1,
      "gagal": 0,
      "dilewati": 12,
      "belum_ada_run_terjadwal": 0
    },
    "sumber": [
      {
        "sumber": "BAPANAS",
        "dataset": "panel_harga",
        "kategori": "DILEWATI",
        "status": "ERROR",
        "pesan": "DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "last_available_date": null,
        "umur_hari": null,
        "run_terakhir": {
          "run_id": "2026-11-24:BAPANAS:panel_harga:MANUAL",
          "crawler": "garda-crawler/0.1.0",
          "crawl_date": "2026-11-24",
          "mulai": "2026-11-24T03:50:04Z",
          "selesai": "2026-11-24T03:50:04Z",
          "status": "ERROR",
          "jumlah_baru": 0,
          "jumlah_diteruskan": 0,
          "last_available_date": null,
          "pesan": "DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
          "pemicu": "MANUAL",
          "dilewati": true
        },
        "menurut_siklus": {
          "status": "ERROR",
          "last_available_date": null,
          "crawled_at": "2026-11-24T03:50:04Z",
          "umur_hari": null,
          "pesan": "DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas"
        }
      },
      {
        "sumber": "BMKG",
        "dataset": "prakiraan_cuaca_adm4",
        "kategori": "SEGAR",
        "status": "FRESH",
        "pesan": "permintaan 23, gagal 0 | ditolak 23 (hari_tidak_lengkap=23)",
        "last_available_date": "2026-11-26",
        "umur_hari": 0,
        "run_terakhir": {
          "run_id": "2026-11-24:BMKG:prakiraan_cuaca_adm4:MANUAL",
          "crawler": "garda-crawler/0.1.0",
          "crawl_date": "2026-11-24",
          "mulai": "2026-11-24T03:49:17Z",
          "selesai": "2026-11-24T03:50:03Z",
          "status": "FRESH",
          "jumlah_baru": 138,
          "jumlah_diteruskan": 0,
          "last_available_date": "2026-11-26",
          "pesan": "permintaan 23, gagal 0 | ditolak 23 (hari_tidak_lengkap=23)",
          "pemicu": "MANUAL",
          "dilewati": false
        },
        "menurut_siklus": {
          "status": "FRESH",
          "last_available_date": "2026-11-26",
          "crawled_at": "2026-11-24T03:50:03Z",
          "umur_hari": -2,
          "pesan": "permintaan 23, gagal 0 | ditolak 23 (hari_tidak_lengkap=23)"
        }
      },
      {
        "sumber": "PIHPS",
        "dataset": "harga_pasar_tradisional",
        "kategori": "BASI",
        "status": "STALE_CARRIED_FORWARD",
        "pesan": "permintaan 23, gagal 0",
        "last_available_date": "2026-11-23",
        "umur_hari": 1,
        "run_terakhir": {
          "run_id": "2026-11-24:PIHPS:harga_pasar_tradisional:MANUAL",
          "crawler": "garda-crawler/0.1.0",
          "crawl_date": "2026-11-24",
          "mulai": "2026-11-24T03:48:30Z",
          "selesai": "2026-11-24T03:49:17Z",
          "status": "STALE_CARRIED_FORWARD",
          "jumlah_baru": 0,
          "jumlah_diteruskan": 31,
          "last_available_date": "2026-11-23",
          "pesan": "permintaan 23, gagal 0",
          "pemicu": "MANUAL",
          "dilewati": false
        },
        "menurut_siklus": {
          "status": "STALE_CARRIED_FORWARD",
          "last_available_date": "2026-11-23",
          "crawled_at": "2026-11-24T03:49:17Z",
          "umur_hari": 1,
          "pesan": "permintaan 23, gagal 0"
        }
      }
    ],
    "sinyal_sumber": [
      {
        "sinyal_kunci": "SUMBER_DATA:BAPANAS:panel_harga",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BAPANAS dilewati: panel_harga",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      }
    ]
  },
  "makro": {
    "tersedia": false,
    "alasan": "TIDAK_ADA_DATA",
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "items": []
  }
};

export const PREDIKSI_DETAIL_C05: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "data_cutoff": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "forecast": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "komoditas_kode": "CBM-KRT",
    "kabupaten_kode": "1171",
    "wilayah_tersedia": [
      "1107",
      "1171",
      "1174"
    ],
    "item": {
      "komoditas_kode": "CBM-KRT",
      "kabupaten_kode": "1171",
      "harga_terakhir": 73750,
      "tanggal_terakhir": "2026-11-23",
      "satuan": "Rp/kg",
      "kurva": [
        {
          "horizon": 7,
          "tanggal": "2026-12-01",
          "p10": 51980.27,
          "p50": 71250,
          "p90": 97663.25,
          "perubahan_p50_pct": -3.4
        },
        {
          "horizon": 14,
          "tanggal": "2026-12-08",
          "p10": 48423.99,
          "p50": 71250,
          "p90": 104835.69,
          "perubahan_p50_pct": -3.4
        },
        {
          "horizon": 30,
          "tanggal": "2026-12-24",
          "p10": 44035.9,
          "p50": 72500,
          "p90": 119362.84,
          "perubahan_p50_pct": -1.7
        }
      ],
      "model": {
        "nama": "seasonal_naive",
        "adalah_baseline": true,
        "fallback": false,
        "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
        "n_kandidat": 5
      },
      "metrik": {
        "mape": 3.796583411667475,
        "rmse": 2894.98840163139,
        "baseline": "seasonal_naive",
        "mape_baseline": 3.796583411667475,
        "mengalahkan_baseline": false,
        "evaluasi": "holdout_lipatan_terakhir",
        "n_lipatan": 5,
        "n_lipatan_seleksi": 4,
        "horizon": [
          7,
          14,
          30
        ]
      },
      "riwayat": [
        {
          "tanggal": "2026-11-11",
          "nilai": 71000
        },
        {
          "tanggal": "2026-11-12",
          "nilai": 70750
        },
        {
          "tanggal": "2026-11-13",
          "nilai": 70250
        },
        {
          "tanggal": "2026-11-16",
          "nilai": 71250
        },
        {
          "tanggal": "2026-11-18",
          "nilai": 68750
        },
        {
          "tanggal": "2026-11-19",
          "nilai": 72500
        },
        {
          "tanggal": "2026-11-20",
          "nilai": 70000
        },
        {
          "tanggal": "2026-11-23",
          "nilai": 73750
        }
      ],
      "penjelasan": {
        "metode": "dekomposisi",
        "satuan_kontribusi": "poin_persen_harga",
        "faktor": [
          {
            "kelompok": "TREN_HARGA_LAPANGAN",
            "label": "Tren harga dan lapangan",
            "kontribusi": -1.6203,
            "arah": "TURUN",
            "fitur": [
              {
                "nama": "tren_jangka_menengah",
                "kontribusi": -5.069
              },
              {
                "nama": "momentum_7_hari",
                "kontribusi": 3.4486
              }
            ]
          },
          {
            "kelompok": "KALENDER",
            "label": "Kalender hari besar",
            "kontribusi": -0.0891,
            "arah": "TURUN",
            "fitur": [
              {
                "nama": "kal_pola_mingguan",
                "kontribusi": -0.0891
              }
            ]
          }
        ],
        "narasi": "Harga Cabai Merah Keriting di wilayah 1171 diproyeksikan turun 1.7% dalam 30 hari (model seasonal_naive). Faktor terbesar: tren harga dan lapangan (menekan turun), kalender hari besar (menekan turun)."
      }
    }
  }
};

export const PREDIKSI_DETAIL_C04: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "data_cutoff": "2026-11-24T04:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "forecast": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "komoditas_kode": "CBM-KRT",
    "kabupaten_kode": "1171",
    "wilayah_tersedia": [
      "1107",
      "1171",
      "1174"
    ],
    "item": {
      "komoditas_kode": "CBM-KRT",
      "kabupaten_kode": "1171",
      "harga_terakhir": 73750,
      "tanggal_terakhir": "2026-11-23",
      "satuan": "Rp/kg",
      "kurva": [
        {
          "horizon": 7,
          "tanggal": "2026-12-01",
          "p10": 51980.27,
          "p50": 71250,
          "p90": 97663.25,
          "perubahan_p50_pct": -3.4
        },
        {
          "horizon": 14,
          "tanggal": "2026-12-08",
          "p10": 48423.99,
          "p50": 71250,
          "p90": 104835.69,
          "perubahan_p50_pct": -3.4
        },
        {
          "horizon": 30,
          "tanggal": "2026-12-24",
          "p10": 44035.9,
          "p50": 72500,
          "p90": 119362.84,
          "perubahan_p50_pct": -1.7
        }
      ],
      "model": {
        "nama": "seasonal_naive",
        "adalah_baseline": true,
        "fallback": false,
        "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
        "n_kandidat": 5
      },
      "metrik": {
        "mape": 3.796583411667475,
        "rmse": 2894.98840163139,
        "baseline": "seasonal_naive",
        "mape_baseline": 3.796583411667475,
        "mengalahkan_baseline": false,
        "evaluasi": "holdout_lipatan_terakhir",
        "n_lipatan": 5,
        "n_lipatan_seleksi": 4,
        "horizon": [
          7,
          14,
          30
        ]
      },
      "riwayat": [
        {
          "tanggal": "2026-11-11",
          "nilai": 71000
        },
        {
          "tanggal": "2026-11-12",
          "nilai": 70750
        },
        {
          "tanggal": "2026-11-13",
          "nilai": 70250
        },
        {
          "tanggal": "2026-11-16",
          "nilai": 71250
        },
        {
          "tanggal": "2026-11-18",
          "nilai": 68750
        },
        {
          "tanggal": "2026-11-19",
          "nilai": 72500
        },
        {
          "tanggal": "2026-11-20",
          "nilai": 70000
        },
        {
          "tanggal": "2026-11-23",
          "nilai": 73750
        }
      ],
      "penjelasan": {
        "metode": "dekomposisi",
        "satuan_kontribusi": null,
        "faktor": [
          {
            "kelompok": "TREN_HARGA_LAPANGAN",
            "label": "Tren harga dan lapangan",
            "kontribusi": -1.6203,
            "arah": "TURUN",
            "fitur": [
              {
                "nama": "tren_jangka_menengah",
                "kontribusi": -5.069
              },
              {
                "nama": "momentum_7_hari",
                "kontribusi": 3.4486
              }
            ]
          },
          {
            "kelompok": "KALENDER",
            "label": "Kalender hari besar",
            "kontribusi": -0.0891,
            "arah": "TURUN",
            "fitur": [
              {
                "nama": "kal_pola_mingguan",
                "kontribusi": -0.0891
              }
            ]
          }
        ],
        "narasi": "Harga Cabai Merah Keriting di wilayah 1171 diproyeksikan turun 1.7% dalam 30 hari (model seasonal_naive). Faktor terbesar: tren harga dan lapangan (menekan turun), kalender hari besar (menekan turun)."
      }
    }
  }
};

export const PREDIKSI_DETAIL_1101: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "data_cutoff": "2026-11-24T04:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "forecast": {
    "tersedia": false,
    "alasan": "WILAYAH_TANPA_DATA",
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T04Z",
    "cycle_timestamp": "2026-11-24T04:00:00Z",
    "komoditas_kode": "CBM-KRT",
    "kabupaten_kode": "1101",
    "wilayah_tersedia": [
      "1107",
      "1171",
      "1174"
    ],
    "item": null
  }
};

export const PREDIKSI_DETAIL_C03: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T03Z",
    "cycle_timestamp": "2026-11-24T03:00:00Z",
    "data_cutoff": "2026-11-24T03:00:00Z",
    "status": "SEBAGIAN",
    "target_inflasi": null,
    "label_target": null
  },
  "forecast": {
    "tersedia": false,
    "alasan": "TIDAK_ADA_DATA",
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T03Z",
    "cycle_timestamp": "2026-11-24T03:00:00Z",
    "komoditas_kode": "CBM-KRT",
    "kabupaten_kode": "1171",
    "wilayah_tersedia": [],
    "item": null
  }
};

export const PREDIKSI_RINGKAS_C05: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "data_cutoff": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "forecast_ringkas": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "kabupaten_kode": "1171",
    "wilayah_tersedia": [
      "1107",
      "1171",
      "1174"
    ],
    "items": [
      {
        "komoditas_kode": "BRS-MED",
        "kabupaten_kode": "1171",
        "harga_terakhir": 15600,
        "tanggal_terakhir": "2026-11-23",
        "satuan": "Rp/kg",
        "kurva": [
          {
            "horizon": 7,
            "tanggal": "2026-12-01",
            "p10": 15399.35,
            "p50": 15600,
            "p90": 15803.26,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 14,
            "tanggal": "2026-12-08",
            "p10": 15325.9,
            "p50": 15600,
            "p90": 15879,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 30,
            "tanggal": "2026-12-24",
            "p10": 15207.48,
            "p50": 15600,
            "p90": 16002.65,
            "perubahan_p50_pct": 0
          }
        ],
        "model": {
          "nama": "naive",
          "adalah_baseline": true,
          "fallback": false,
          "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
          "n_kandidat": 5
        },
        "metrik": {
          "mape": 0.3383190883191131,
          "rmse": 69.24500897299257,
          "baseline": "naive",
          "mape_baseline": 0.3383190883191131,
          "mengalahkan_baseline": false,
          "evaluasi": "holdout_lipatan_terakhir",
          "n_lipatan": 5,
          "n_lipatan_seleksi": 4,
          "horizon": [
            7,
            14,
            30
          ]
        }
      },
      {
        "komoditas_kode": "BRS-PRM",
        "kabupaten_kode": "1171",
        "harga_terakhir": 17250,
        "tanggal_terakhir": "2026-11-23",
        "satuan": "Rp/kg",
        "kurva": [
          {
            "horizon": 7,
            "tanggal": "2026-12-01",
            "p10": 17017.04,
            "p50": 17250,
            "p90": 17486.15,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 14,
            "tanggal": "2026-12-08",
            "p10": 16931.81,
            "p50": 17250,
            "p90": 17574.17,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 30,
            "tanggal": "2026-12-24",
            "p10": 16794.42,
            "p50": 17250,
            "p90": 17717.94,
            "perubahan_p50_pct": 0
          }
        ],
        "model": {
          "nama": "naive",
          "adalah_baseline": true,
          "fallback": false,
          "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
          "n_kandidat": 5
        },
        "metrik": {
          "mape": 0,
          "rmse": 0,
          "baseline": "naive",
          "mape_baseline": 0,
          "mengalahkan_baseline": false,
          "evaluasi": "holdout_lipatan_terakhir",
          "n_lipatan": 5,
          "n_lipatan_seleksi": 4,
          "horizon": [
            7,
            14,
            30
          ]
        }
      },
      {
        "komoditas_kode": "BWM",
        "kabupaten_kode": "1171",
        "harga_terakhir": 31250,
        "tanggal_terakhir": "2026-11-23",
        "satuan": "Rp/kg",
        "kurva": [
          {
            "horizon": 7,
            "tanggal": "2026-12-01",
            "p10": 28402.51,
            "p50": 31250,
            "p90": 34382.96,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 14,
            "tanggal": "2026-12-08",
            "p10": 27417.83,
            "p50": 31250,
            "p90": 35617.79,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 30,
            "tanggal": "2026-12-24",
            "p10": 25892.29,
            "p50": 31250,
            "p90": 37716.34,
            "perubahan_p50_pct": 0
          }
        ],
        "model": {
          "nama": "naive",
          "adalah_baseline": true,
          "fallback": false,
          "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
          "n_kandidat": 5
        },
        "metrik": {
          "mape": 4.699074074074089,
          "rmse": 1544.627825494401,
          "baseline": "naive",
          "mape_baseline": 4.699074074074089,
          "mengalahkan_baseline": false,
          "evaluasi": "holdout_lipatan_terakhir",
          "n_lipatan": 5,
          "n_lipatan_seleksi": 4,
          "horizon": [
            7,
            14,
            30
          ]
        }
      },
      {
        "komoditas_kode": "BWP",
        "kabupaten_kode": "1171",
        "harga_terakhir": 35750,
        "tanggal_terakhir": "2026-11-23",
        "satuan": "Rp/kg",
        "kurva": [
          {
            "horizon": 7,
            "tanggal": "2026-12-01",
            "p10": 33749.73,
            "p50": 35750,
            "p90": 37868.82,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 14,
            "tanggal": "2026-12-08",
            "p10": 33315.81,
            "p50": 35750,
            "p90": 38362.04,
            "perubahan_p50_pct": 0
          },
          {
            "horizon": 30,
            "tanggal": "2026-12-24",
            "p10": 31497.89,
            "p50": 34500,
            "p90": 37788.24,
            "perubahan_p50_pct": -3.5
          }
        ],
        "model": {
          "nama": "seasonal_naive",
          "adalah_baseline": true,
          "fallback": false,
          "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
          "n_kandidat": 5
        },
        "metrik": {
          "mape": 1.107226107226108,
          "rmse": 519.3375672974065,
          "baseline": "seasonal_naive",
          "mape_baseline": 1.107226107226108,
          "mengalahkan_baseline": false,
          "evaluasi": "holdout_lipatan_terakhir",
          "n_lipatan": 5,
          "n_lipatan_seleksi": 4,
          "horizon": [
            7,
            14,
            30
          ]
        }
      }
    ]
  },
  "model": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
    "klasifikasi": {
      "model": "lightgbm",
      "f1": 0.576271186440678,
      "roc_auc": 0.4094746716697936,
      "brier": 0.25107863899268434,
      "brier_baseline": 0.2490694492065993,
      "n_uji": 93,
      "kalibrasi": "sigmoid",
      "layak": false,
      "kelayakan": "TIDAK_LAYAK",
      "alasan": "ROC-AUC holdout 0.41 < 0.60; Brier 0.251 tidak lebih rendah dari baseline laju dasar 0.249"
    },
    "regresi": {
      "pemenang": {
        "holt_winters": 0,
        "lightgbm_quantile": 0,
        "naive": 22,
        "sarimax": 0,
        "seasonal_naive": 8
      },
      "mape_median": 0.9698231810367397,
      "rmse_median": 363.4839432971472,
      "mape_baseline_median": 0.9698231810367397,
      "n_seri": 30,
      "mengalahkan_baseline": 0,
      "n_dievaluasi_holdout": 30,
      "n_dilewati": 0,
      "n_gagal": 0,
      "model_global": "lightgbm_quantile",
      "tingkat": "FIT_ULANG"
    }
  }
};

export const PREDIKSI_SIKLUS_SALAH: LampiranMlMentah = {
  "tersedia": false,
  "alasan": "CYCLE_ID_TIDAK_VALID",
  "data_utama": "POSTGRES",
  "forecast_ringkas": {
    "tersedia": false,
    "alasan": "CYCLE_ID_TIDAK_VALID",
    "kabupaten_kode": null,
    "wilayah_tersedia": [],
    "items": []
  },
  "model": {
    "tersedia": false,
    "alasan": "CYCLE_ID_TIDAK_VALID",
    "status": "",
    "target_inflasi": null,
    "label_target": null,
    "klasifikasi": null,
    "regresi": null
  }
};

export const SPASIAL_WILAYAH_C05: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "data_cutoff": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "wilayah": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "items": [
      {
        "wilayah_kode": "1107",
        "tingkat": "KABUPATEN",
        "komoditas_forecast": [
          "BRS-MED",
          "BRS-PRM",
          "BWM",
          "BWP",
          "CBM-KRT",
          "DAG-AYM",
          "DAG-SPI",
          "GLA",
          "MYK",
          "TLR"
        ],
        "inflasi": {
          "kelas": null,
          "kelayakan": "TIDAK_LAYAK",
          "probabilitas": 0.449812,
          "skor_tekanan": null,
          "siklus": "",
          "target": "INDEKS_PANGAN",
          "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
        },
        "sinyal_aktif": {
          "aktif": 1,
          "kritis": 0,
          "siaga": 1,
          "waspada": 0,
          "info": 0
        }
      },
      {
        "wilayah_kode": "1171",
        "tingkat": "KABUPATEN",
        "komoditas_forecast": [
          "BRS-MED",
          "BRS-PRM",
          "BWM",
          "BWP",
          "CBM-KRT",
          "DAG-AYM",
          "DAG-SPI",
          "GLA",
          "MYK",
          "TLR"
        ],
        "inflasi": {
          "kelas": null,
          "kelayakan": "TIDAK_LAYAK",
          "probabilitas": 0.485315,
          "skor_tekanan": null,
          "siklus": "",
          "target": "INDEKS_PANGAN",
          "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
        },
        "sinyal_aktif": {
          "aktif": 0,
          "kritis": 0,
          "siaga": 0,
          "waspada": 0,
          "info": 0
        }
      },
      {
        "wilayah_kode": "1174",
        "tingkat": "KABUPATEN",
        "komoditas_forecast": [
          "BRS-MED",
          "BRS-PRM",
          "BWM",
          "BWP",
          "CBM-KRT",
          "DAG-AYM",
          "DAG-SPI",
          "GLA",
          "MYK",
          "TLR"
        ],
        "inflasi": {
          "kelas": null,
          "kelayakan": "TIDAK_LAYAK",
          "probabilitas": 0.49869,
          "skor_tekanan": null,
          "siklus": "",
          "target": "INDEKS_PANGAN",
          "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
        },
        "sinyal_aktif": {
          "aktif": 0,
          "kritis": 0,
          "siaga": 0,
          "waspada": 0,
          "info": 0
        }
      }
    ]
  }
};

export const EWS_C05: LampiranMlMentah = {
  "tersedia": true,
  "data_utama": "POSTGRES",
  "siklus": {
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "data_cutoff": "2026-11-24T05:00:00Z",
    "status": "BERHASIL",
    "target_inflasi": "INDEKS_PANGAN",
    "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
  },
  "ews": {
    "tersedia": true,
    "sumber_data": "DATASAFE",
    "cycle_id": "C20261124T05Z",
    "cycle_timestamp": "2026-11-24T05:00:00Z",
    "ringkasan": {
      "aktif": 13,
      "kritis": 0,
      "siaga": 1,
      "waspada": 0,
      "info": 12
    },
    "sinyal": [
      {
        "sinyal_kunci": "ANOMALI_HARGA:DAG-AYM:1107:TURUN",
        "severity": "SIAGA",
        "jenis": "ANOMALI_HARGA",
        "komoditas_kode": "DAG-AYM",
        "kabupaten_kode": "1107",
        "nilai": -17.297,
        "ambang": 10,
        "metode": "zscore_musiman",
        "judul": "Anomali harga Daging Ayam Ras Segar (turun)",
        "pesan": "Harga Daging Ayam Ras Segar di wilayah 1107 pada 2026-11-23 menyimpang -17.3% dari pola musimannya (z=-3.6).",
        "aktif": true,
        "menjadi_alert": true,
        "alert": null
      },
      {
        "sinyal_kunci": "SUMBER_DATA:BAPANAS:panel_harga",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BAPANAS dilewati: panel_harga",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: API Panel Harga (api-panelhargav2.badanpangan.go.id) mensyaratkan API key (401 'Invalid or missing API key'); tindakan: minta API key resmi ke Bapanas",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      },
      {
        "sinyal_kunci": "SUMBER_DATA:BMKG:peringatan_cuaca",
        "severity": "INFO",
        "jenis": "SUMBER_DATA",
        "komoditas_kode": null,
        "kabupaten_kode": null,
        "nilai": null,
        "ambang": 0,
        "metode": "kesegaran_sumber",
        "judul": "Sumber BMKG dilewati: peringatan_cuaca",
        "pesan": "Status ERROR, data asli terakhir tidak diketahui. DILEWATI: robots.txt www.bmkg.go.id melarang /alerts dan /api (diperiksa 2026-11-24); tindakan: minta akses resmi data peringatan cuaca ke BMKG",
        "aktif": true,
        "menjadi_alert": false,
        "alert": null
      }
    ],
    "terpotong": false
  }
};
