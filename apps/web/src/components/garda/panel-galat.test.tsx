import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/lib/api/client";
import { PanelGalat, pesanGalatMuat } from "./panel-galat";

describe("pesanGalatMuat", () => {
  it("galat HTTP memakai pesan server (berbahasa Indonesia)", () => {
    assert.equal(pesanGalatMuat(new ApiError(500, "Gagal mengambil data KPI")), "Gagal mengambil data KPI");
  });

  it("galat lain (jaringan putus, null) memakai petunjuk sambungan, bukan pesan teknis", () => {
    assert.match(pesanGalatMuat(new TypeError("Failed to fetch")), /Periksa sambungan internet/);
    assert.match(pesanGalatMuat(null), /Periksa sambungan internet/);
    assert.match(pesanGalatMuat(undefined), /Periksa sambungan internet/);
  });
});

describe("PanelGalat", () => {
  it("peringatan dengan judul dan pesan; tombol coba lagi hanya bila ada pemuat ulang", async () => {
    let dicoba = 0;
    const { rerender } = render(<PanelGalat judul="Peta panas harga gagal dimuat" error={new ApiError(500, "boom")} onCoba={() => dicoba++} />);

    const alert = screen.getByRole("alert");
    assert.match(alert.textContent ?? "", /Peta panas harga gagal dimuat/);
    assert.match(alert.textContent ?? "", /boom/);
    await userEvent.setup({ pointerEventsCheck: 0 }).click(screen.getByRole("button", { name: /Coba lagi/ }));
    assert.equal(dicoba, 1);

    rerender(<PanelGalat judul="Peta panas harga gagal dimuat" />);
    assert.equal(screen.queryByRole("button"), null);
  });
});
