import { cn } from "@/lib/utils";

/**
 * Latar khas Garda: garis kontur topografi (dataran tinggi Gayo hingga pesisir)
 * di atas kabut dingin, plus cahaya ambient yang mengikuti siklus ekonomi aktif.
 * Dirender di server, deterministik, tanpa JS klien.
 */
function ring(cx: number, cy: number, r: number, i: number, seed: number) {
  const N = 72;
  const pts: [number, number][] = [];
  for (let k = 0; k < N; k++) {
    const t = (k / N) * Math.PI * 2;
    const rr = r * (1 + 0.13 * Math.sin(3 * t + seed + i * 0.18) + 0.07 * Math.sin(5 * t + seed * 1.7 - i * 0.11) + 0.035 * Math.sin(8 * t + seed * 2.3));
    pts.push([cx + Math.cos(t) * rr * 1.25, cy + Math.sin(t) * rr * 0.85]);
  }
  // Catmull-Rom tertutup → kurva Bézier kubik
  let d = `M${pts[0][0].toFixed(1)},${pts[0][1].toFixed(1)}`;
  for (let k = 0; k < N; k++) {
    const p0 = pts[(k - 1 + N) % N], p1 = pts[k], p2 = pts[(k + 1) % N], p3 = pts[(k + 2) % N];
    const c1 = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6];
    const c2 = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6];
    d += ` C${c1[0].toFixed(1)},${c1[1].toFixed(1)} ${c2[0].toFixed(1)},${c2[1].toFixed(1)} ${p2[0].toFixed(1)},${p2[1].toFixed(1)}`;
  }
  return `${d} Z`;
}

const PUSAT = [
  { cx: 1180, cy: 180, n: 11, seed: 0.7 },
  { cx: 260, cy: 760, n: 9, seed: 2.1 },
  { cx: 860, cy: 980, n: 7, seed: 4.2 },
];

const PATHS = PUSAT.flatMap((p) => Array.from({ length: p.n }, (_, i) => ring(p.cx, p.cy, 34 + i * 38, i, p.seed)));

export function KonturBackdrop({ className }: { className?: string }) {
  return (
    <div aria-hidden className={cn("pointer-events-none fixed inset-0 -z-10 overflow-hidden", className)}>
      <div className="absolute inset-0 bg-[radial-gradient(120%_80%_at_80%_-10%,#e6eef9_0%,transparent_55%),radial-gradient(90%_70%_at_0%_100%,#e3f4f2_0%,transparent_60%)] dark:bg-[radial-gradient(120%_80%_at_80%_-10%,#13234a_0%,transparent_55%),radial-gradient(90%_70%_at_0%_100%,#06302f_0%,transparent_60%)]" />
      {/* cahaya ambient siklus */}
      <div className="animate-aurora absolute -top-40 left-1/3 h-[420px] w-[720px] rounded-full blur-3xl" style={{ background: "var(--siklus-glow)" }} />
      <svg className="absolute inset-0 h-full w-full text-navy-900/[0.055] dark:text-white/[0.035]" viewBox="0 0 1440 1000" preserveAspectRatio="xMidYMid slice">
        <g fill="none" stroke="currentColor" strokeWidth={1}>
          {PATHS.map((d, i) => (
            <path key={i} d={d} />
          ))}
        </g>
      </svg>
    </div>
  );
}
