import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { screen } from "@testing-library/react";
import { renderHalaman } from "@/test/render";
import { MeterKekuatanSandi } from "./meter-kekuatan-sandi";

const meter = () => screen.getByRole("meter", { name: "Kekuatan kata sandi" });

describe("MeterKekuatanSandi", () => {
  it("isian kosong: meter tidak tampil sama sekali", () => {
    renderHalaman(<MeterKekuatanSandi sandi="" />);
    assert.equal(screen.queryByRole("meter"), null);
    assert.equal(screen.queryByText("Lemah"), null);
  });

  it("belum memenuhi aturan: Lemah, nilai 1 dari 3, satu segmen terisi", () => {
    const { container } = renderHalaman(<MeterKekuatanSandi sandi="abc" />);
    const m = meter();
    assert.equal(m.getAttribute("aria-valuemin"), "0");
    assert.equal(m.getAttribute("aria-valuemax"), "3");
    assert.equal(m.getAttribute("aria-valuenow"), "1");
    assert.equal(m.getAttribute("aria-valuetext"), "Lemah");
    assert.ok(screen.getByText("Lemah"));
    assert.equal(container.querySelectorAll("[class*='bg-defisit']").length, 1);
  });

  it("memenuhi semua aturan (kurang dari 12 karakter): Sedang, nilai 2, dua segmen terisi", () => {
    const { container } = renderHalaman(
      <MeterKekuatanSandi sandi="Abcdef1!" />,
    );
    const m = meter();
    assert.equal(m.getAttribute("aria-valuenow"), "2");
    assert.equal(m.getAttribute("aria-valuetext"), "Sedang");
    assert.ok(screen.getByText("Sedang"));
    assert.equal(container.querySelectorAll("[class*='bg-waspada']").length, 2);
  });

  it("memenuhi semua aturan dan sekurangnya 12 karakter: Kuat, nilai 3, semua segmen terisi", () => {
    const { container } = renderHalaman(
      <MeterKekuatanSandi sandi="Abcdefghij1!" />,
    );
    const m = meter();
    assert.equal(m.getAttribute("aria-valuenow"), "3");
    assert.equal(m.getAttribute("aria-valuetext"), "Kuat");
    assert.ok(screen.getByText("Kuat"));
    assert.equal(container.querySelectorAll("[class*='bg-normal']").length, 3);
  });

  it("label teks berada di dalam elemen meter dan mengikuti perubahan isian", () => {
    const { rerender } = renderHalaman(<MeterKekuatanSandi sandi="a" />);
    assert.match(meter().textContent ?? "", /Lemah/);
    rerender(<MeterKekuatanSandi sandi="Abcdef1!" />);
    assert.match(meter().textContent ?? "", /Sedang/);
    rerender(<MeterKekuatanSandi sandi="" />);
    assert.equal(screen.queryByRole("meter"), null);
  });
});
