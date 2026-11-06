// ESLint flat config apps/web.
//
// Saat ini hanya menegakkan guardrail form (docs/FORM.md): tidak ada kiriman form native yang me-reload
// halaman atau meninggalkan POST di riwayat browser ("Confirm Form Resubmission"). Aturan rekomendasi
// Next/React/TypeScript belum diaktifkan; pluginnya didaftarkan supaya komentar `eslint-disable` yang sudah
// ada di kode tetap dikenali, dan aturannya dapat diaktifkan bertahap.
import nextPlugin from "@next/eslint-plugin-next";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

/** Pola sintaks terlarang beserta alasannya; diekspor untuk diuji (src/lib/form/guardrail.test.ts). */
export const ATURAN_FORM = [
  {
    selector: "JSXOpeningElement[name.name='form']",
    message:
      "Gunakan <FormBff> (components/form/form-bff.tsx) dengan useFormBff: <form> mentah bisa terkirim native (reload/POST di riwayat).",
  },
  {
    selector: "JSXOpeningElement[name.name='button']:not(:has(JSXAttribute[name.name='type']))",
    message:
      "Isi type pada <button>: tanpa type, tombol di dalam form menjadi tombol kirim (mis. \"Batal\" ikut mengirim form).",
  },
  {
    selector: "JSXAttribute[name.name='formAction']",
    message:
      "formAction memicu kiriman form native atau Server Action tanpa pembungkus. Panggil BFF lewat useFormBff, atau Server Action lewat useActionState/useTransition.",
  },
];

export default tseslint.config(
  {
    ignores: [
      ".next/**",
      "node_modules/**",
      "public/**",
      "playwright-report/**",
      "test-results/**",
      "next-env.d.ts",
    ],
  },
  {
    files: ["**/*.{ts,tsx,mts,mjs}"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    plugins: {
      "@typescript-eslint": tseslint.plugin,
      "react-hooks": reactHooks,
      "@next/next": nextPlugin,
    },
    // Aturan plugin belum aktif, jadi komentar eslint-disable yang ada akan terlapor "tak terpakai".
    linterOptions: { reportUnusedDisableDirectives: "off" },
    rules: {
      "no-restricted-syntax": ["error", ...ATURAN_FORM],
    },
  },
);
