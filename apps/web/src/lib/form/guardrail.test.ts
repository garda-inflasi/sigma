/**
 * Guardrail form: aturan ESLint (eslint.config.mjs) menolak kiriman form native, dan seluruh kode aplikasi
 * mematuhinya. Dijalankan bersama `pnpm test` supaya pelanggaran tertangkap walau `pnpm lint` tidak dijalankan.
 */
import assert from "node:assert/strict";
import path from "node:path";
import { before, describe, it } from "node:test";
import { ESLint } from "eslint";

const akarWeb = path.resolve(import.meta.dirname, "../../..");
let eslint: ESLint;

before(() => {
  eslint = new ESLint({ cwd: akarWeb });
});

async function pelanggaran(kode: string): Promise<string[]> {
  const [hasil] = await eslint.lintText(kode, { filePath: path.join(akarWeb, "src/contoh-guardrail.tsx") });
  return hasil.messages.filter((m) => m.ruleId === "no-restricted-syntax").map((m) => m.message);
}

describe("guardrail form (ESLint)", () => {
  it("<form action method=POST> mentah ditolak", async () => {
    const pesan = await pelanggaran(
      `export const A = () => <form action="/api/auth/login" method="POST"><button type="submit">Kirim</button></form>;`,
    );
    assert.equal(pesan.length, 1);
    assert.match(pesan[0], /FormBff/);
  });

  it("<form onSubmit> mentah juga ditolak (handler bisa lupa preventDefault)", async () => {
    assert.equal((await pelanggaran(`export const A = () => <form onSubmit={() => {}} />;`)).length, 1);
  });

  it("formAction ditolak", async () => {
    const pesan = await pelanggaran(
      `export const A = ({ aksi }: { aksi: () => void }) => <button type="submit" formAction={aksi}>Kirim</button>;`,
    );
    assert.equal(pesan.length, 1);
    assert.match(pesan[0], /useActionState|useFormBff/);
  });

  it("<button> tanpa type ditolak (di dalam form ia menjadi tombol kirim)", async () => {
    const pesan = await pelanggaran(`export const A = () => <button onClick={() => {}}>Batal</button>;`);
    assert.equal(pesan.length, 1);
    assert.match(pesan[0], /type/);
    assert.deepEqual(await pelanggaran(`export const A = () => <button type="button" onClick={() => {}}>Batal</button>;`), []);
  });

  it("<FormBff> diterima", async () => {
    assert.deepEqual(
      await pelanggaran(
        `import { FormBff } from "@/components/form/form-bff";\nexport const A = () => <FormBff onKirim={() => {}}><button type="submit">Kirim</button></FormBff>;`,
      ),
      [],
    );
  });

  it("seluruh kode aplikasi mematuhi guardrail", async () => {
    const hasil = await eslint.lintFiles(["src/**/*.{ts,tsx}"]);
    const salah = hasil.flatMap((r) =>
      r.messages
        .filter((m) => m.ruleId === "no-restricted-syntax" || m.fatal)
        .map((m) => `${path.relative(akarWeb, r.filePath)}:${m.line} ${m.message}`),
    );
    assert.deepEqual(salah, []);
  });
});
