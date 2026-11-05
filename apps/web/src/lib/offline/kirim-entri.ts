import { ApiError, apiFetch, MODE_MOCK } from "@/lib/api/client";
import { toKirimHargaBody, toVersiServer } from "@/lib/api/lapangan";
import { sleep } from "@/lib/utils";
import type { EntriLokal } from "@/lib/stores/field-queue-store";

export type HasilKirim =
  | { ok: true }
  | { konflik: true; versiServer: NonNullable<EntriLokal["versiServer"]> };

/**
 * Mengirim satu entri ke POST /v1/lapangan/harga. Idempotency-Key = localId, jadi
 * kiriman ulang aman. 409 menjadi hasil konflik beserta versi server; galat lain dilempar
 * agar mesin sinkron mencatatnya sebagai GAGAL. Mode mock (tanpa backend) hanya mensimulasikan kiriman.
 */
export async function kirimEntri(e: EntriLokal): Promise<HasilKirim> {
  if (MODE_MOCK) {
    await sleep(450);
    return { ok: true };
  }
  try {
    await apiFetch("/v1/lapangan/harga", {
      method: "POST",
      body: JSON.stringify(toKirimHargaBody(e)),
      idempotencyKey: e.localId,
    });
    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const versiServer = toVersiServer(err.payload);
      if (versiServer) return { konflik: true, versiServer };
    }
    throw err;
  }
}
