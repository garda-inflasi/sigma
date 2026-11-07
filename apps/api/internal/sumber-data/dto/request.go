package dto

// BulkSumberDataRequest: setiap entri IDs adalah UUID sumber atau kodenya (mis. SRC-BI, alias lama); entri yang
// tidak ada dihitung gagal.
type BulkSumberDataRequest struct {
	Action string   `json:"action" binding:"required,oneof=jeda lanjutkan"`
	IDs    []string `json:"ids" binding:"required,min=1,max=100,dive,required,max=64"`
}
