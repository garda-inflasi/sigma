/** Tanda Garda: perisai membulat dengan kurva harga yang "dijinakkan" kembali ke koridor. */
export function BrandMark({ className = "size-9" }: { className?: string }) {
  return (
    <svg viewBox="0 0 36 36" className={className} aria-hidden>
      <defs>
        <linearGradient id="gm-bg" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#233e7a" />
          <stop offset="1" stopColor="#0b1630" />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="34" height="34" rx="11" fill="url(#gm-bg)" />
      <path d="M8 22.5h20" stroke="#33c2b8" strokeOpacity=".35" strokeWidth="1.2" strokeDasharray="2 2.4" />
      <path d="M8 24c3-.4 4.2-8.8 7.4-9.3 3.1-.5 3.4 6.3 6.6 6.5 2.4.1 3.9-2.2 6-2.3" fill="none" stroke="#67dbd0" strokeWidth="2.2" strokeLinecap="round" />
      <circle cx="28" cy="18.9" r="2.1" fill="#fff" />
    </svg>
  );
}
