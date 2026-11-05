import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { safeInternalPath } from "./safe-path";

describe("safeInternalPath", () => {
  it("menerima path internal beserta query", () => {
    assert.equal(safeInternalPath("/kpi?x=1"), "/kpi?x=1");
    assert.equal(safeInternalPath("/data/harga"), "/data/harga");
  });

  it("menolak URL absolut, protokol-relatif, dan backslash", () => {
    for (const bad of [
      "https://evil.com",
      "//evil.com",
      "/\\evil.com",
      "javascript:alert(1)",
      "evil",
      "",
      "/a\nb",
    ]) {
      assert.equal(safeInternalPath(bad), "/", bad);
    }
    assert.equal(safeInternalPath(undefined, "/eksekutif"), "/eksekutif");
  });
});
