/** Tanda Garda: kotak membulat near-black dengan kurva harga yang "dijinakkan" kembali ke koridor. */
export function BrandMark({ className = "size-8" }: { className?: string }) {
  return (
    <svg viewBox="0 0 36 36" className={className} aria-hidden>
      <rect x="1" y="1" width="34" height="34" rx="9" fill="#18181b" />
      <path d="M8 22.5h20" stroke="#5eead4" strokeOpacity=".4" strokeWidth="1.2" strokeDasharray="2 2.4" />
      <path d="M8 24c3-.4 4.2-8.8 7.4-9.3 3.1-.5 3.4 6.3 6.6 6.5 2.4.1 3.9-2.2 6-2.3" fill="none" stroke="#2dd4bf" strokeWidth="2.2" strokeLinecap="round" />
      <circle cx="28" cy="18.9" r="2.1" fill="#fff" />
    </svg>
  );
}
