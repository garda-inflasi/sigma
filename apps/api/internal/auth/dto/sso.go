package dto

// SSOStatusResponse memberi tahu klien penyedia SSO mana yang aktif (dibaca halaman masuk di server).
type SSOStatusResponse struct {
	Google bool `json:"google"`
}

// SSOAuthorizeResponse adalah awal alur SSO: URL tujuan pengalihan browser dan binding yang disimpan klien
// dalam cookie httpOnly lalu dikirim kembali pada callback. Binding tidak boleh sampai ke URL atau skrip halaman.
type SSOAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
	Binding      string `json:"binding"`
}

// SSOCallbackRequest adalah parameter yang dibawa penyedia (code, state) beserta binding dari cookie browser.
type SSOCallbackRequest struct {
	Code    string `json:"code" binding:"required,max=2048"`
	State   string `json:"state" binding:"required,max=128"`
	Binding string `json:"binding" binding:"required,max=128"`
}
