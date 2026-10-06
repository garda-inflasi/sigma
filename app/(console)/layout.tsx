import { redirect } from "next/navigation";
import { ConsoleShell } from "@/components/layout/console-shell";
import { getSession } from "@/lib/auth/session";

export default async function ConsoleLayout({ children }: { children: React.ReactNode }) {
  const user = await getSession();
  if (!user) redirect("/masuk");
  return <ConsoleShell>{children}</ConsoleShell>;
}
