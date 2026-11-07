import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import type { AuditIntegritasResponse } from "@/lib/api/audit";
import {
  amplop,
  galatApi,
  pasangFetchPalsu,
  renderHalaman,
  type FetchPalsu,
} from "@/test/render";
import { AuditIntegritasCard } from "./audit-integritas-card";

const HASH = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08";

const valid: AuditIntegritasResponse = {
  verifikasi_terakhir: {
    id: "12",
    mulai: "2026-11-07T03:00:00Z",
    selesai: "2026-11-07T03:00:04Z",
    pemicu: "JADWAL",
    status: "VALID",
    jumlah_baris: 12345,
    jumlah_masalah: 0,
    id_awal: "1",
    id_akhir: "12345",
    hash_akhir: HASH,
  },
  partisi: [
    {
      nama: "audit_log_p202611",
      dari: "2026-11-01T00:00:00Z",
      sampai: "2026-12-01T00:00:00Z",
      perkiraan_baris: 1234,
    },
    {
      nama: "audit_log_p202610",
      dari: "2026-10-01T00:00:00Z",
      sampai: "2026-11-01T00:00:00Z",
      perkiraan_baris: null,
    },
  ],
  arsip: [],
};

const rusak: AuditIntegritasResponse = {
  ...valid,
  verifikasi_terakhir: {
    ...valid.verifikasi_terakhir!,
    pemicu: "MANUAL",
    status: "RUSAK",
    jumlah_masalah: 1,
    masalah_pertama: {
      id: "4301",
      jenis: "ISI_BERUBAH",
      keterangan: "hash tersimpan tidak sama dengan isi baris",
    },
  },
};

let api: FetchPalsu;
afterEach(() => api?.pulihkan());

function pasang(data: unknown) {
  api = pasangFetchPalsu({ "GET /v1/audit/integritas": data });
}

describe("AuditIntegritasCard", () => {
  it("VALID: lencana utuh, jumlah baris, hash ringkas, dan ringkasan partisi", async () => {
    pasang(amplop(valid));
    renderHalaman(<AuditIntegritasCard onLihatBaris={() => {}} />, {
      url: "/audit",
    });

    assert.ok(screen.getByRole("status", { name: "Memuat status integritas" }));
    assert.ok(await screen.findByText("Utuh"));
    assert.ok(screen.getByText("Rantai hash utuh: 12.345 baris diverifikasi."));
    const hash = screen.getByText(`${HASH.slice(0, 12)}…`);
    assert.equal(hash.getAttribute("title"), HASH);
    assert.ok(screen.getByText(/2 partisi bulanan aktif/));
    assert.ok(screen.getByText(/Okt 2026 – Nov 2026/));
    assert.ok(screen.getByText(/terjadwal/));
    assert.equal(screen.queryByRole("button"), null);
    assert.equal(screen.queryByText(/diarsipkan/), null);
    assert.equal(api.ke("GET", "/v1/audit/integritas").length, 1);
  });

  it("RUSAK: menyebut baris bermasalah dan tombol membuka detail barisnya", async () => {
    pasang(amplop(rusak));
    const dibuka: string[] = [];
    const { user } = renderHalaman(
      <AuditIntegritasCard onLihatBaris={(id) => dibuka.push(id)} />,
      { url: "/audit" },
    );

    assert.ok(await screen.findByText("Rusak"));
    assert.ok(
      screen.getByText(
        "Terdeteksi perubahan pada baris #4301 (isi baris berubah).",
      ),
    );
    assert.ok(screen.getByText(/manual/));

    await user.click(screen.getByRole("button", { name: /Lihat baris #4301/ }));
    assert.deepEqual(dibuka, ["4301"]);
  });

  it("GAGAL: menampilkan keterangan galat tanpa tombol baris", async () => {
    pasang(
      amplop({
        ...valid,
        verifikasi_terakhir: {
          ...valid.verifikasi_terakhir!,
          status: "GAGAL",
          keterangan: "context deadline exceeded",
        },
      }),
    );
    renderHalaman(<AuditIntegritasCard onLihatBaris={() => {}} />, {
      url: "/audit",
    });

    assert.ok(await screen.findByText("Verifikasi gagal"));
    assert.ok(
      screen.getByText(
        "Verifikasi gagal dijalankan: context deadline exceeded",
      ),
    );
    assert.equal(screen.queryByRole("button"), null);
  });

  it("belum pernah diverifikasi: lencana netral dan arsip terbaru disebut bila ada", async () => {
    pasang(
      amplop({
        verifikasi_terakhir: null,
        partisi: valid.partisi,
        arsip: [
          {
            id: "1",
            partisi: "audit_log_p202101",
            dari: "2021-01-01T00:00:00Z",
            sampai: "2021-02-01T00:00:00Z",
            jumlah_baris: 100,
            id_pertama: "1",
            id_terakhir: "100",
            berkas: "audit_log_p202101.jsonl.gz",
            sha256: HASH,
            diarsipkan_pada: "2026-02-01T02:00:00Z",
          },
        ],
      }),
    );
    renderHalaman(<AuditIntegritasCard onLihatBaris={() => {}} />, {
      url: "/audit",
    });

    assert.ok(await screen.findByText("Belum diverifikasi"));
    assert.ok(screen.getByText(/belum pernah dijalankan/));
    assert.ok(screen.getByText(/1 partisi diarsipkan/));
    assert.ok(screen.getByText("audit_log_p202101.jsonl.gz"));
  });

  it("galat endpoint: hanya pesan redup, tanpa merusak halaman", async () => {
    pasang(galatApi(500, "boom"));
    renderHalaman(<AuditIntegritasCard onLihatBaris={() => {}} />, {
      url: "/audit",
    });

    await waitFor(() =>
      assert.ok(screen.getByText("Status integritas tidak tersedia")),
    );
    assert.equal(screen.queryByRole("button"), null);
    assert.equal(screen.queryByText("Utuh"), null);
  });
});
