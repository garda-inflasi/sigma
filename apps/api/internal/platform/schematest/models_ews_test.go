package schematest

import (
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
)

func init() {
	registerModels("ews", &ewsdomain.EwsAlert{}, &ewsdomain.EwsRiwayat{}, &ewsdomain.EwsEvent{}, &ewsdomain.EwsNotifikasi{}, &ewsdomain.PreferensiNotifikasi{}, &ewsdomain.LanggananPush{}, &ewsdomain.AturanEws{}, &ewsdomain.AturanEwsRiwayat{})
}
