import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { labelKode } from "./kode";

describe("labelKode", () => {
  it("memakai kode tampilan bila ada", () => {
    assert.equal(labelKode({ id: "1", kode: "EWS-0901" }), "EWS-0901");
    assert.equal(
      labelKode({
        id: "f2356c7b-bb80-3d01-84c3-ba32f27a2910",
        kode: "INT-301",
      }),
      "INT-301",
    );
  });

  it("jatuh ke id untuk baris tanpa kode (null atau tidak ada)", () => {
    assert.equal(labelKode({ id: "42" }), "42");
    assert.equal(labelKode({ id: "42", kode: undefined }), "42");
    assert.equal(labelKode({ id: "42", kode: null }), "42");
  });
});
