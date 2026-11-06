import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { screen, within } from "@testing-library/react";
import { renderHalaman } from "@/test/render";
import {
  STATE_AWAL,
  type KunciFilter,
  type StateTabelPengguna,
} from "@/lib/pengguna/tabel";
import { FilterPengguna } from "./filter-pengguna";

function pasang(state: StateTabelPengguna = STATE_AWAL) {
  const filter: [KunciFilter, string | undefined][] = [];
  let reset = 0;
  const r = renderHalaman(
    <FilterPengguna
      state={state}
      onFilter={(k, v) => filter.push([k, v])}
      onReset={() => reset++}
    />,
  );
  return { ...r, filter, reset: () => reset };
}

describe("FilterPengguna", () => {
  it("menampilkan lima kontrol berlabel dan tanpa tombol reset saat tidak ada filter", () => {
    pasang();
    const grup = screen.getByRole("group", { name: "Filter pengguna" });
    for (const label of [
      "Filter peran",
      "Filter lapis",
      "Filter status",
      "Filter mfa",
      "Filter kunci login",
    ]) {
      assert.ok(within(grup).getByRole("combobox", { name: label }), label);
    }
    assert.equal(screen.queryByRole("button", { name: /Reset filter/ }), null);
  });

  it("nilai filter aktif tampil di kontrolnya dan tombol reset menghitungnya", async () => {
    const { user, reset } = pasang({
      ...STATE_AWAL,
      status: "MENUNGGU",
      mfa: "true",
    });

    assert.match(
      screen.getByRole("combobox", { name: "Filter status" }).textContent ?? "",
      /Menunggu aktivasi/,
    );
    assert.match(
      screen.getByRole("combobox", { name: "Filter mfa" }).textContent ?? "",
      /MFA aktif/,
    );
    // Kontrol lain tetap pada "semua" (placeholder-nya adalah labelnya sendiri).
    assert.match(
      screen.getByRole("combobox", { name: "Filter peran" }).textContent ?? "",
      /Peran/,
    );

    await user.click(screen.getByRole("button", { name: "Reset filter (2)" }));
    assert.equal(reset(), 1);
  });

  it("memilih opsi memanggil onFilter dengan kunci dan nilai; 'semua' melepas filter", async () => {
    const { user, filter } = pasang({ ...STATE_AWAL, status: "AKTIF" });

    await user.click(
      screen.getByRole("combobox", {
        name: "Filter kunci login",
      }),
    );
    await user.click(await screen.findByRole("option", { name: "Terkunci" }));
    assert.deepEqual(filter.at(-1), ["terkunci", "true"]);

    await user.click(screen.getByRole("combobox", { name: "Filter status" }));
    await user.click(
      await screen.findByRole("option", { name: "Status: semua" }),
    );
    assert.deepEqual(filter.at(-1), ["status", undefined]);
  });

  it("opsi peran memakai label peran, nilainya id enum", async () => {
    const { user, filter } = pasang();

    await user.click(screen.getByRole("combobox", { name: "Filter peran" }));
    await user.click(
      await screen.findByRole("option", { name: "Administrator Sistem" }),
    );

    assert.deepEqual(filter.at(-1), ["peran", "ADMIN_SISTEM"]);
  });
});
