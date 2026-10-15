import { redirect } from "next/navigation";
import { getSession } from "@/lib/auth/session";
import { ROLES } from "@/lib/rbac/roles";

export default async function Home() {
  const user = await getSession();
  redirect(user ? ROLES[user.role].beranda : "/masuk");
}
