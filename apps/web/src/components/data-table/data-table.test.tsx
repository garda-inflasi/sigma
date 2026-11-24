import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { screen, waitFor } from "@testing-library/react";
import { renderHalaman } from "@/test/render";
import { DataTable } from "./data-table";

const BARIS = Array.from({ length: 30 }, (_, i) => ({
  id: String(i),
  nama: i === 0 ? "Unik" : `Baris ${i}`,
}));

const tabel = () => (
  <DataTable
    data={BARIS}
    columns={[{ accessorKey: "nama", header: "Nama" }]}
    getRowId={(r) => r.id}
    entityLabel="baris"
    pageSize={10}
    searchPlaceholder="Cari baris"
  />
);

describe("DataTable", () => {
  it("kembali ke halaman yang ada saat hasil pencarian menyusut", async () => {
    const { user } = renderHalaman(tabel());
    await user.click(screen.getByRole("button", { name: "Halaman berikutnya" }));
    assert.ok(screen.getByText("2 / 3"));

    await user.type(screen.getByRole("textbox", { name: "Cari baris" }), "Unik");

    await waitFor(() => assert.ok(screen.getByText("1 / 1")));
    assert.ok(screen.getByText("Unik"));
  });

  it("tombol reset menghapus pencarian dan menampilkan semua baris lagi", async () => {
    const { user } = renderHalaman(tabel());
    await user.type(screen.getByRole("textbox", { name: "Cari baris" }), "Unik");
    await waitFor(() => assert.ok(screen.getByText("1 / 1")));

    await user.click(screen.getByRole("button", { name: /Reset filter \(1\)/ }));

    await waitFor(() => assert.ok(screen.getByText("1 / 3")));
    assert.equal(screen.queryByRole("button", { name: /Reset filter/ }), null);
  });
});
