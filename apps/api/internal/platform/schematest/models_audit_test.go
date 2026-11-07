package schematest

import (
	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
)

func init() {
	registerModels("audit", &auditdomain.AuditLog{}, &auditdomain.Verifikasi{}, &auditdomain.Arsip{}, &auditdomain.SegmenArsipRow{})
}
