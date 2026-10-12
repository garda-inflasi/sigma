# Alur sistem & data — AI Garda Inflasi

Diagram Mermaid (dirender otomatis di GitHub/GitLab). Versi beranimasi ada di aplikasi: `/alur-sistem`.

## 1. Arsitektur 4 lapis

```mermaid
flowchart LR
  subgraph L1[Layer 1 · Ingesti data]
    A1[PWA lapangan<br/>offline-first] --- A2[API BPS · BI/PIHPS · DJPb]
    A2 --- A3[BMKG · OpenWeather] --- A4[Scraper Bapanas · portal daerah]
  end
  subgraph L2[Layer 2 · Analitik AI]
    B1[Proyeksi harga & pasokan] --- B2[EWS & anomali]
    B2 --- B3[Spasial & rute] --- B4[Shelf-life & food loss]
  end
  subgraph L3[Layer 3 · Dashboard & DSS]
    C1[Operasional] --- C2[Eksekutif] --- C3[Policy brief otomatis]
  end
  subgraph L4[Layer 4 · Intervensi]
    D1[Siklus normal] --- D2[Siklus tekanan] --- D3[Siklus surplus]
  end
  L1 --> L2 --> L3 --> L4
  L4 -. evaluasi KPI .-> L3
```

## 2. Pipeline medallion

```mermaid
flowchart LR
  S[(Sumber<br/>lapangan · PIHPS · BPS · DJPb · BMKG · scraper)] --> B[Bronze<br/>mentah + lineage ID]
  B --> Sv[Silver<br/>bersih · dedup · normalisasi satuan & kode BPS · enrichment cuaca]
  Sv --> G[Gold<br/>agregat wilayah/waktu · fitur model · hanya terpublikasi]
  G --> M[Model ML] & E[EWS] & D[Dashboard] & P[Policy brief]
  Sv -. validasi lintas sumber .-> X{Selisih > ambang?}
  X -- ya --> E
```

## 3. Entri harga offline → publikasi

```mermaid
sequenceDiagram
  autonumber
  actor Agen as Agen mahasiswa
  participant PWA as PWA (IndexedDB)
  participant SW as Service worker
  participant API as API Go
  participant DB as PostgreSQL
  actor Koor as Koordinator
  actor Risk as Unit Risiko/Data
  Agen->>PWA: Isi form (harga, foto, GPS, kode etik)
  PWA->>PWA: Cek rentang ±25%, ekstrem, duplikasi, geofence
  PWA-->>Agen: Tersimpan (offline/online)
  Note over PWA,SW: Saat online / tiap 60 dtk / event "garda:sinkron"
  PWA->>API: POST /v1/lapangan/harga (Idempotency-Key = localId)
  alt bentrok dengan entri lain
    API-->>PWA: 409 + versiServer
    PWA-->>Agen: Status KONFLIK → pilih versi
  else sukses
    API->>DB: INSERT entri (DIAJUKAN) + audit
    API-->>PWA: 201
  end
  Koor->>API: POST /v1/harga/bulk {aksi: validasi}
  API->>DB: DIVALIDASI (validator ≠ penginput)
  Risk->>API: POST /v1/harga/bulk {aksi: publikasi}
  API->>DB: DIPUBLIKASI → masuk gold layer
```

```mermaid
stateDiagram-v2
  [*] --> DRAF: dicatat di perangkat
  DRAF --> DIAJUKAN: sinkron + cek otomatis
  DIAJUKAN --> DIVALIDASI: koordinator
  DIAJUKAN --> DITOLAK: koordinator
  DITOLAK --> DRAF: survei ulang
  DIVALIDASI --> DIPUBLIKASI: Unit Risiko / Data
  DIPUBLIKASI --> [*]
```

## 4. Tiga siklus intervensi

```mermaid
stateDiagram-v2
  direction LR
  [*] --> N

  state "Siklus normal" as N {
    [*] --> n1
    n1: Pantau rutin data produsen
    n2: Verifikasi silang BPS
    n3: Optimasi jadwal pengiriman
    n4: Agregasi volume reguler
    n1 --> n2
    n2 --> n3
    n3 --> n4
    n4 --> n1
  }

  state "Siklus tekanan harga / defisit" as T {
    [*] --> t1
    t1: EWS terpicu
    t2: Verifikasi cepat BI/BPS
    t3: Petakan suplai sentra surplus
    t4: Usulan logistik kilat
    t5: Dukungan instrumen belanja
    t1 --> t2
    t2 --> t3
    t3 --> t4
    t4 --> t5
    t5 --> t1
    note right of t3
      Perlu persetujuan
      Dewan Pengarah
    end note
    note right of t5
      Perlu dukungan
      Kanwil DJPb
    end note
  }

  state "Siklus surplus" as S {
    [*] --> s1
    s1: Deteksi over-supply
    s2: Arahkan ke cold storage
    s3: Rekomendasi pengolahan turunan
    s4: Ekspansi pembeli antardaerah
    s1 --> s2
    s2 --> s3
    s3 --> s4
    s4 --> s1
  }

  n2 --> t1: EWS kritis/siaga
  s4 --> n1: harga kembali ke koridor
  n4 --> s1: over-supply terdeteksi
  t5 --> n1: surplus terserap
```

## 5. Dari peringatan ke keputusan

```mermaid
sequenceDiagram
  participant Model as Model EWS
  participant Redis as Redis pub/sub
  participant Go as API Go (SSE)
  participant UI as Konsol (toast + feed)
  actor Data as Unit Data / BI / BPS
  actor Prog as Manajer Program
  actor Dewan as Dewan Pengarah / DJPb
  Model->>Redis: alert (severity, skor dampak)
  Redis->>Go: publish
  Go-->>UI: event "alert" (SSE)
  UI->>UI: cache TanStack + toast berdenyut (Siaga/Kritis)
  Data->>Go: Akui & verifikasi
  Prog->>Go: Buat kartu intervensi (rekomendasi AI)
  Dewan->>Go: Setujui tahap bergembok
  Go-->>UI: kartu maju, audit tercatat
  Note over UI: Dampak dievaluasi di KPI & policy brief berikutnya
```

## 6. Autentikasi & otorisasi

```mermaid
flowchart LR
  U[Pengguna] --> SSO[SSO institusi + MFA] --> Auth[Layanan auth Go]
  Auth -->|JWT HS256 di cookie httpOnly garda_session| Proxy[proxy.ts<br/>gerbang rute RBAC]
  Proxy -->|izin cukup| Page[Halaman]
  Proxy -->|tidak| Deny[/akses-ditolak/]
  Page --> API[API Go<br/>RBAC + cakupan baris + audit]
```
