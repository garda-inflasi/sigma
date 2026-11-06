# Konvensi form (apps/web)

Semua form dikirim **asinkron ke BFF** (Route Handler Next.js di `app/api/*`, atau API Go lewat `apiFetch`).
Tidak ada form yang dikirim browser secara native, jadi tidak ada reload halaman, tidak ada POST di riwayat browser,
dan tidak ada dialog "Confirm Form Resubmission".

## Latar belakang

Form login dulu memakai `<form method="POST" action="/api/auth/login">`. Rute BFF menjawab kegagalan dengan
`NextResponse.redirect()`, yang bawaannya **307**: metode POST dipertahankan, sehingga halaman `/masuk?error=…`
yang tampil adalah hasil POST. Me-reload halaman itu memicu dialog kirim ulang form di Safari/Chrome.

## Aturan

1. Pakai `<FormBff>` (`components/form/form-bff.tsx`), bukan `<form>` mentah. Komponen ini tidak menerima
   `action`/`method`, selalu `noValidate`, dan memanggil `preventDefault()` walau handler-nya lupa.
2. Pakai `useFormBff` (`lib/form/use-form-bff.ts`) untuk state kiriman:
   - validasi skema zod di klien → galat per field (`galatField`);
   - `pending` (tombol nonaktif, kiriman ganda diabaikan);
   - galat 400/422 dari server → galat per field (`{ errors: { field: pesan } }`) dan galat form (`galat`);
   - galat lain → `galat` (bawaan: pesan server untuk 4xx, pesan umum untuk 5xx/jaringan);
   - navigasi lewat `useRouter` (`replace` bawaan, atau `push`) **hanya sesudah 2xx**. Tujuan selalu disaring
     menjadi path internal (cegah open redirect dan URL `javascript:`).
3. Kiriman yang mengubah cookie sesi (masuk, OTP, ganti kata sandi, MFA) mengembalikan `{ ke, segarkan: true }`.
   Sesi dibaca root layout, yang tidak dirender ulang oleh navigasi antargrup rute; `router.refresh()` memuatnya ulang.
4. Rute BFF untuk form menerima **JSON** dan menjawab **JSON** (data atau galat dengan kode stabil). Rute BFF
   tidak boleh menjawab kiriman form dengan redirect.
5. Validator yang sudah ada (mis. `validasiBuat`) dipakai lewat `skemaDariValidasi` (`lib/form/skema.ts`),
   bukan ditulis ulang.
6. Server Action: proyek ini tidak memakainya. Bila kelak dipakai, panggil dari Client Component lewat
   `useActionState`/`useTransition` (bukan `<form action={serverAction}>` atau `formAction`), dengan aturan yang
   sama: galat tampil di form, navigasi hanya sesudah sukses.
7. Tombol di dalam form wajib jelas jenisnya: `<button>` mentah harus punya `type`; `Button` (shadcn) dan
   `SpringButton` bawaannya `type="button"`, jadi tombol kirim menulis `type="submit"` secara eksplisit. Tombol
   aksi kedua (mis. "Simpan draf") memanggil `form.kirimNilai(...)`.

## Contoh

```tsx
const skema = z.object({ email: z.string().trim().min(1, "Email wajib diisi.") });

const form = useFormBff<z.infer<typeof skema>, { tujuan: string }>({
  skema,
  kirim: (n) => bffFetch("/api/contoh", { method: "POST", body: JSON.stringify(n) }),
  sukses: (hasil) => ({ ke: hasil.tujuan }),
  judulToastGagal: "Gagal menyimpan",
});

<FormBff onKirim={form.kirimDari(() => ({ email }))} sibuk={form.pending}>
  <Input aria-invalid={!!form.galatField.email} aria-describedby={idGalatField("email")} … />
  <PesanField field="email" pesan={form.galatField.email} />
  {form.galat && <p role="alert">{form.galat.pesan}</p>}
  <button type="submit" disabled={form.pending}>Simpan</button>
</FormBff>
```

Mutasi React Query cukup dibungkus: `kirim: (n) => mutasi.mutateAsync(n)`. Bila hook mutasi sudah menampilkan
toast galat, jangan isi `judulToastGagal` supaya toast tidak ganda.

## Guardrail

- **ESLint** (`apps/web/eslint.config.mjs`, `pnpm -C apps/web lint`): `no-restricted-syntax` menolak `<form>` mentah,
  atribut `formAction`, dan `<button>` tanpa `type`. Satu-satunya pengecualian `<form>` adalah `FormBff` sendiri.
- **Tes** (`src/lib/form/guardrail.test.ts`, ikut `pnpm test`): memastikan aturan ESLint menangkap
  `<form action method="POST">`/`formAction` dan seluruh `src/` mematuhinya.
- **E2E** (`apps/web/e2e/fixtures.ts`): setiap navigasi dokumen non-GET ke aplikasi menggagalkan tes apa pun.
  `auth.spec.ts` juga menguji muat ulang sesudah login gagal adalah GET biasa.
