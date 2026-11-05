import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { inisialNama } from "./nama";

describe("inisialNama", () => {
  it("melewati gelar di depan dan di belakang nama", () => {
    assert.equal(inisialNama("Prof. Dr.rer.nat. Ilham Maulana, S.Si."), "IM");
    assert.equal(inisialNama("Dr. Nurul Aini, S.E., M.E."), "NA");
    assert.equal(inisialNama("Ir. Cut Putri, M.Ec.Dev"), "CP");
    assert.equal(inisialNama("Rina Safitri, S.T., M.T."), "RS");
  });

  it("nama satu kata dan nama tanpa gelar", () => {
    assert.equal(inisialNama("Drs. Syahrul, M.M."), "S");
    assert.equal(inisialNama("Zulkifli, S.P."), "Z");
    assert.equal(inisialNama("aulia rahman"), "AR");
    assert.equal(inisialNama("Super Admin"), "SA");
  });

  it("hanya gelar atau kosong tidak menghasilkan inisial palsu", () => {
    assert.equal(inisialNama("Dr."), "D");
    assert.equal(inisialNama(""), "");
  });
});
