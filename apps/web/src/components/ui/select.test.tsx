import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { screen } from "@testing-library/react";
import { useState } from "react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { renderHalaman } from "@/test/render";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./select";

function ContohSelect({ awal = "1171" }: { awal?: string }) {
  const [nilai, setNilai] = useState(awal);
  return (
    <Select value={nilai} onValueChange={setNilai}>
      <SelectTrigger aria-label="Pilih wilayah">
        <SelectValue placeholder="Pilih wilayah…" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="1171">Kota Banda Aceh</SelectItem>
        <SelectItem value="1174">Kota Lhokseumawe</SelectItem>
        <SelectItem value="1106">Kabupaten Aceh Tengah</SelectItem>
      </SelectContent>
    </Select>
  );
}

describe("Select (searchable & soft style)", () => {
  it("menampilkan label nilai awal sebelum dibuka dan menyaring opsi lewat pencarian", async () => {
    const { user } = renderHalaman(<ContohSelect />);
    const trigger = screen.getByRole("combobox", { name: "Pilih wilayah" });
    assert.match(trigger.textContent ?? "", /Kota Banda Aceh/);

    await user.click(trigger);
    const search = await screen.findByRole("textbox", { name: "Cari pilihan…" });
    await user.type(search, "Tengah");

    assert.equal(screen.queryByRole("option", { name: "Kota Banda Aceh" }), null);
    const match = screen.getByRole("option", { name: "Kabupaten Aceh Tengah" });
    await user.click(match);

    assert.match(trigger.textContent ?? "", /Kabupaten Aceh Tengah/);
  });

  it("menampilkan pesan kosong saat pencarian tidak cocok dan mendukung Enter untuk memilih", async () => {
    const { user } = renderHalaman(<ContohSelect />);
    await user.click(screen.getByRole("combobox", { name: "Pilih wilayah" }));
    const search = await screen.findByRole("textbox", { name: "Cari pilihan…" });

    await user.type(search, "tidakada");
    assert.ok(screen.getByText("Tidak ada pilihan yang cocok."));

    await user.click(screen.getByRole("button", { name: "Bersihkan pencarian" }));
    await user.type(search, "Lhokseumawe{Enter}");
    assert.match(screen.getByRole("combobox", { name: "Pilih wilayah" }).textContent ?? "", /Kota Lhokseumawe/);
  });

  it("kolom pencarian dapat diketik di dalam Dialog tanpa dicuri FocusScope", async () => {
    const { user } = renderHalaman(
      <Dialog open>
        <DialogContent>
          <DialogTitle>Uji dialog</DialogTitle>
          <ContohSelect />
        </DialogContent>
      </Dialog>,
    );

    await user.click(screen.getByRole("combobox", { name: "Pilih wilayah" }));
    const search = await screen.findByRole("textbox", { name: "Cari pilihan…" });
    await user.type(search, "Lhok");
    await user.click(screen.getByRole("option", { name: "Kota Lhokseumawe" }));
    assert.match(screen.getByRole("combobox", { name: "Pilih wilayah" }).textContent ?? "", /Kota Lhokseumawe/);
  });
});
