"use client";

import { ChevronsUpDown, UserCog } from "lucide-react";
import { useRouter } from "next/navigation";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useSession } from "@/lib/auth/session-context";
import { DEV_ROLE_COOKIE } from "@/lib/auth/resolve";
import { LAPIS_META, ROLES, ROLE_IDS, type Lapis, type Role } from "@/lib/rbac/roles";

/** Pengalih peran — hanya di lingkungan pengembangan, untuk meninjau RBAC seluruh halaman. */
export function RoleSwitcher() {
  const { role } = useSession();
  const router = useRouter();
  if (process.env.NODE_ENV === "production") return null;

  const pilih = (r: string) => {
    document.cookie = `${DEV_ROLE_COOKIE}=${r}; path=/; max-age=2592000; samesite=lax`;
    router.push(ROLES[r as Role].beranda);
    router.refresh();
  };
  const perLapis = (Object.keys(LAPIS_META) as Lapis[]).map((l) => ({ l, roles: ROLE_IDS.filter((r) => ROLES[r].lapis === l) }));

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="glass-subtle hidden h-9 items-center gap-2 rounded-md px-3 text-xs font-medium text-muted-foreground hover:text-foreground xl:inline-flex">
        <UserCog className="size-4" />
        Lihat sebagai: <span className="text-foreground">{ROLES[role].label}</span>
        <ChevronsUpDown className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-[70vh] w-80 overflow-y-auto rounded-xl p-1.5">
        <DropdownMenuRadioGroup value={role} onValueChange={pilih}>
          {perLapis.map(({ l, roles }, i) => (
            <div key={l}>
              {i > 0 && <DropdownMenuSeparator />}
              <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">{LAPIS_META[l].label}</DropdownMenuLabel>
              {roles.map((r) => (
                <DropdownMenuRadioItem key={r} value={r} className="rounded-lg">
                  <div>
                    <p className="text-sm">{ROLES[r].label}</p>
                    <p className="text-xs text-muted-foreground">{ROLES[r].lembaga}</p>
                  </div>
                </DropdownMenuRadioItem>
              ))}
            </div>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
