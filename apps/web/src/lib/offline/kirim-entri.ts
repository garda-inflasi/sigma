import { ApiError, apiFetch, MODE_MOCK } from "@/lib/api/client";
import { toKirimHargaBody, toVersiServer } from "@/lib/api/lapangan";
import { sleep } from "@/lib/utils";
import type { EntriLokal } from "@/lib/stores/field-queue-store";

export type HasilKirim =
  | { ok: true }
  | { konflik: true; versiServer: NonNullable<EntriLokal["versiServer"]> };

const MIME_EXT: Record<string, string> = {
  "image/jpeg": "jpg",
  "image/png": "png",
  "image/webp": "webp",
};

export function dataUrlToBlob(dataUrl: string): { blob: Blob; ext: string } | null {
  const match = /^data:([^;]+);base64,(.+)$/.exec(dataUrl);
  if (!match) return null;
  const mime = match[1];
  const base64 = match[2];
  const binStr = atob(base64);
  const bytes = new Uint8Array(binStr.length);
  for (let i = 0; i < binStr.length; i++) {
    bytes[i] = binStr.charCodeAt(i);
  }
  const ext = MIME_EXT[mime] ?? "jpg";
  return { blob: new Blob([bytes], { type: mime }), ext };
}

/**
 * Mengirim satu entri ke POST /v1/lapangan/harga. Idempotency-Key = localId, jadi
 * kiriman ulang aman. Bila ada foto terkompresi, diunggah menyusul ke
 * POST /v1/lapangan/harga/:id/foto. 409 menjadi hasil konflik beserta versi server;
 * galat lain dilempar agar mesin sinkron mencatatnya sebagai GAGAL.
 */
export async function kirimEntri(e: EntriLokal): Promise<HasilKirim> {
  if (MODE_MOCK) {
    await sleep(450);
    return { ok: true };
  }
  try {
    const res = await apiFetch<{ id?: string } | undefined>("/v1/lapangan/harga", {
      method: "POST",
      body: JSON.stringify(toKirimHargaBody(e)),
      idempotencyKey: e.localId,
    });

    if (e.foto && res?.id) {
      const parsed = dataUrlToBlob(e.foto);
      if (parsed) {
        const fd = new FormData();
        fd.append("foto", parsed.blob, `bukti-${e.localId}.${parsed.ext}`);
        await apiFetch(`/v1/lapangan/harga/${encodeURIComponent(res.id)}/foto`, {
          method: "POST",
          body: fd,
        });
      }
    }

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const versiServer = toVersiServer(err.payload);
      if (versiServer) return { konflik: true, versiServer };
    }
    throw err;
  }
}
