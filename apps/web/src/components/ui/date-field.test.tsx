import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { FormBff } from "@/components/form/form-bff";
import { renderHalaman } from "@/test/render";
import { DateField, DateTimeField } from "./date-field";

function Contoh({
  awal = "",
  min,
  max,
  onNilai,
  onKirim,
}: {
  awal?: string;
  min?: string;
  max?: string;
  onNilai?: (v: string) => void;
  onKirim?: () => void;
}) {
  const [nilai, setNilai] = useState(awal);
  return (
    <FormBff onKirim={() => onKirim?.()}>
      <label htmlFor="uji">Tenggat</label>
      <DateField
        id="uji"
        value={nilai}
        min={min}
        max={max}
        onValueChange={(v) => {
          setNilai(v);
          onNilai?.(v);
        }}
      />
      <button type="submit">Simpan</button>
    </FormBff>
  );
}

describe("DateField", () => {
  it("mengubah ketikan hh/bb/tttt menjadi nilai ISO", async () => {
    const nilai: string[] = [];
    const { user } = renderHalaman(<Contoh onNilai={(v) => nilai.push(v)} />);
    const field = screen.getByLabelText("Tenggat") as HTMLInputElement;
    await user.type(field, "24112026");
    assert.equal(field.value, "24/11/2026");
    assert.deepEqual(nilai, ["2026-11-24"]);
  });

  it("menahan kiriman form selama ketikan tidak valid", async () => {
    let terkirim = 0;
    const nilai: string[] = [];
    const { user } = renderHalaman(
      <Contoh
        awal="2026-11-24"
        onNilai={(v) => nilai.push(v)}
        onKirim={() => terkirim++}
      />,
    );
    const field = screen.getByLabelText("Tenggat");
    await user.clear(field);
    await user.type(field, "31022026");
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    assert.equal(terkirim, 0);
    assert.match(screen.getByRole("alert").textContent ?? "", /hh\/bb\/tttt/);
    assert.equal(field.getAttribute("aria-invalid"), "true");
    assert.deepEqual(nilai, [""]);
  });

  it("memilih hari dari kalender lalu menutupnya", async () => {
    const nilai: string[] = [];
    const { user } = renderHalaman(
      <Contoh awal="2026-11-24" onNilai={(v) => nilai.push(v)} />,
    );
    await user.click(screen.getByRole("button", { name: "Buka kalender" }));
    const dialog = await screen.findByRole("dialog", { name: "Pilih tanggal" });
    const hari = [...dialog.querySelectorAll("button")].find(
      (b) => b.textContent === "12",
    );
    assert.ok(hari);
    await user.click(hari);
    assert.deepEqual(nilai, ["2026-11-12"]);
    await waitFor(() => assert.equal(screen.queryByRole("dialog"), null));
    assert.equal(
      (screen.getByLabelText("Tenggat") as HTMLInputElement).value,
      "12/11/2026",
    );
  });

  it("menonaktifkan hari di luar batas dan menolak ketikannya", async () => {
    const nilai: string[] = [];
    const { user } = renderHalaman(
      <Contoh
        awal="2026-11-24"
        min="2026-11-10"
        max="2026-11-25"
        onNilai={(v) => nilai.push(v)}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Buka kalender" }));
    const dialog = await screen.findByRole("dialog", { name: "Pilih tanggal" });
    const tombol = [
      ...dialog.querySelectorAll("td button"),
    ] as HTMLButtonElement[];
    assert.equal(tombol.find((b) => b.textContent === "9")?.disabled, true);
    assert.equal(tombol.find((b) => b.textContent === "20")?.disabled, false);
    await user.keyboard("{Escape}");
    const field = screen.getByLabelText("Tenggat");
    await user.clear(field);
    await user.type(field, "09112026");
    assert.deepEqual(nilai, [""]);
  });
});

describe("DateTimeField", () => {
  it("mempertahankan jam saat tanggal diganti", async () => {
    const nilai: string[] = [];
    function Waktu() {
      const [v, setV] = useState("2026-11-24T07:05:09");
      return (
        <>
          <label htmlFor="waktu">Waktu</label>
          <DateTimeField
            id="waktu"
            value={v}
            onValueChange={(n) => {
              setV(n);
              nilai.push(n);
            }}
          />
        </>
      );
    }
    const { user } = renderHalaman(<Waktu />);
    assert.equal(
      (screen.getByLabelText("Waktu") as HTMLInputElement).value,
      "24/11/2026 07:05:09",
    );
    await user.click(screen.getByRole("button", { name: "Buka kalender" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Pilih tanggal dan waktu",
    });
    const hari = [...dialog.querySelectorAll("td button")].find(
      (b) => b.textContent === "12",
    );
    assert.ok(hari);
    await user.click(hari);
    assert.deepEqual(nilai, ["2026-11-12T07:05:09"]);
  });
});
