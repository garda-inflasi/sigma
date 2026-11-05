package usecase

import "slices"

// RolesWithPermission mengembalikan peran yang memegang izin, dalam urutan baku peran.
// Dipakai modul lain yang perlu memilih penerima berdasarkan izin (mis. notifikasi EWS).
func RolesWithPermission(permission string) []string {
	var roles []string
	for _, role := range roleOrder {
		if slices.Contains(rolePermissions[role], permission) {
			roles = append(roles, role)
		}
	}
	return roles
}
