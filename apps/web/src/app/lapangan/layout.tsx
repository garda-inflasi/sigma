import { redirect } from "next/navigation";
import { FieldShell } from "@/components/lapangan/field-shell";
import { getSession } from "@/lib/auth/session";

export const metadata = { title: "Lapangan" };

export default async function LapanganLayout({ children }: { children: React.ReactNode }) {
  const user = await getSession();
  if (!user) redirect("/masuk?lanjut=/lapangan/input");
  return <FieldShell>{children}</FieldShell>;
}
