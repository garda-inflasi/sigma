#!/usr/bin/env bash
# Komponen primitif shadcn/ui yang dipakai design system Garda.
# Komponen bertema Garda (GlassCard, SpringButton, DataTable, dll.) ada di components/garda.
set -euo pipefail
npx shadcn@latest add -y \
  avatar badge checkbox command dialog dropdown-menu input label \
  popover progress select separator sheet tabs textarea tooltip alert-dialog
