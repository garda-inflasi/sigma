@AGENTS.md



1. Scraping (bot) -> IHK (BPS), PHIS (BI) -> Object (Pangan, Kondisi (Bencana/Iklim))
- link source data:

    1.⁠ IHK ⁠harga-harga hingga level kabupaten/kota: https://www.bps.go.id/id/statistics-table?subject=536&sortBy=date%2Ctitle&sortOrder=desc%2Casc
    2.⁠ IHK ⁠pertanian, kehutanan, perikana level provinsi: https://www.bps.go.id/id/statistics-table?subject=557
    3.⁠ IHK ⁠Biaya tenaga kerja hingga level provinsi: https://www.bps.go.id/id/statistics-table?subject=537
    4.⁠ IHK ⁠PDRB dan Pertumbuhan Ekonomi hingga level kabupaten/kota: https://www.bps.go.id/id/statistics-table?subject=531
    5.⁠ IHK ⁠Konsumsi dan pendatan: https://www.bps.go.id/id/statistics-table?subject=523
    6. BI
    7. BI
    8. BI dst..
    9. Kemendag https://sp2kp.kemendag.go.id/statistik/variant
    10. NTP


2. simpan data ke database vector (time series)
3. di latih,
4. save vector db, tapi beda documents
    - step 1 - 4 menggunakan Python

5. Backend (Golang) -> api
6. Frontend (NextJS/Typescript) -> UI

7. Integrasi dengan backend
