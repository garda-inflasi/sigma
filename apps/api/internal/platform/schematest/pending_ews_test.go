package schematest

func init() {
	markPending("ews", "ews_alert: kolom level (tipe level_ews tidak ada), skor, deskripsi, wilayah_kode, direkomendasikan_pada, ditindaklanjuti_pada tidak ada; id int64 vs VARCHAR (slice 10-A)")
}
