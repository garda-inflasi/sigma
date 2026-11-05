package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// jawabAkunTerkunci menjawab 423 ACCOUNT_LOCKED dengan akhir kunci, sisa detik, dan header Retry-After.
// err harus membawa usecase.AkunTerkunciError (selalu begitu bila errors.Is(err, ErrAccountLocked)).
func jawabAkunTerkunci(c *gin.Context, err error) {
	var terkunci *usecase.AkunTerkunciError
	if !errors.As(err, &terkunci) {
		response.Error(c, http.StatusLocked, dto.KodeAkunTerkunci, dto.PesanAkunTerkunci)
		return
	}
	sisa := terkunci.SisaDetik
	c.Header("Retry-After", strconv.Itoa(sisa))
	response.ErrorWithData(c, http.StatusLocked, dto.KodeAkunTerkunci, dto.PesanAkunTerkunci, dto.AkunTerkunciData{
		TerkunciSampai: terkunci.Sampai.UTC(),
		SisaDetik:      sisa,
	})
}

// POST /v1/users/:id/buka-kunci
// Membuka kunci login satu pengguna (pengguna.kelola) dan menghapus hitungan percobaan gagalnya. Tercatat
// di audit (UNLOCK_USER). Idempoten: akun yang tidak terkunci dijawab 200 dengan terkunci_sebelumnya false.
func (h *Handler) BukaKunci(c *gin.Context) {
	id := c.Param("id")

	res, err := h.uc.BukaKunciPengguna(audit.RequestContext(c), id, c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			response.NotFound(c, "Pengguna tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal membuka kunci akun")
		return
	}

	response.Success(c, dto.BukaKunciResponse{ID: id, TerkunciSebelumnya: res.TerkunciSebelumnya})
}

// penggunaDenganKunci memetakan pengguna ke DTO beserta akhir kunci login (terkunci_sampai) bagi yang sedang terkunci.
func (h *Handler) penggunaDenganKunci(c *gin.Context, users []domain.Pengguna) ([]dto.UserResponse, error) {
	items := dto.ToUserResponses(users)
	terkunci, err := h.uc.KunciPengguna(c.Request.Context(), users)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if sampai, ok := terkunci[items[i].ID]; ok {
			sampai = sampai.UTC()
			items[i].TerkunciSampai = &sampai
		}
	}
	return items, nil
}
