package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"gorm.io/gorm"
)

// Undang pengguna lewat email (02-D). Alur:
//
//  1. Admin (pengguna.kelola) membuat akun berstatus MENUNGGU TANPA kata sandi, beserta satu undangan: token
//     acak 256-bit yang hanya disimpan sebagai hash SHA-256 (undangan_pengguna), berlaku 72 jam.
//  2. Email berisi tautan /aktivasi?token=... dikirim ke alamat akun.
//  3. Pemilik email membuka tautan dan menetapkan kata sandi (AktifkanAkun): token dipakai, kata sandi
//     disimpan, status AKTIF, tanpa kewajiban ganti kata sandi, semuanya dalam satu transaksi. Sesudahnya ia
//     masuk seperti biasa; aktivasi tidak menerbitkan sesi (login memegang aturan lockout dan MFA).
//
// Anti-enumerasi: akun MENUNGGU tanpa kata sandi berperilaku persis seperti email tak terdaftar di Login
// (hash palsu, 401 INVALID_CREDENTIALS, hitungan lockout per email yang sama) dan ForgotPassword tidak
// mengirim apa pun untuknya, jadi tidak ada jawaban publik yang membedakan akun yang menunggu dari yang tidak ada.
// Token tidak bisa ditebak (256 bit), sehingga membedakan "kedaluwarsa" dari "tidak valid" hanya memberi tahu
// pemegang tautan yang sah.

// UndanganTTLBawaan adalah masa berlaku tautan undangan.
const UndanganTTLBawaan = 72 * time.Hour

// BatasWaktuEmail membatasi lama menunggu SMTP untuk satu email undangan. Pengiriman tanpa batas waktu
// (net/smtp tidak punya) menggantung permintaan admin bila SMTP bermasalah.
const BatasWaktuEmail = 10 * time.Second

// batasWaktuEmail adalah BatasWaktuEmail yang berlaku; variabel agar tes tidak menunggu 10 detik.
var batasWaktuEmail = BatasWaktuEmail

// Panjang maksimum isian akun.
const (
	maksNama    = 255
	maksLembaga = 255
	maksEmail   = 254
	// maksSandiByte adalah batas bcrypt: lebih panjang dari ini ditolak GenerateFromPassword.
	maksSandiByte = 72
	maksTokenKar  = 512
)

var (
	// ErrUndanganInvalid: tautan tidak ada, sudah dipakai, dicabut (diganti undangan baru), atau akunnya tidak
	// lagi menunggu aktivasi.
	ErrUndanganInvalid = errors.New("tautan undangan tidak valid atau sudah dipakai")
	// ErrUndanganKedaluwarsa: tautan masih ada tetapi masa berlakunya habis.
	ErrUndanganKedaluwarsa = errors.New("tautan undangan sudah kedaluwarsa")
	// ErrBukanMenunggu: kirim ulang undangan hanya untuk akun yang menunggu aktivasi.
	ErrBukanMenunggu = domain.ErrBukanMenunggu
	// ErrAkunMenunggu: akun menunggu aktivasi hanya berubah lewat tautan undangan atau dihapus.
	ErrAkunMenunggu = errors.New("akun menunggu aktivasi: statusnya hanya berubah lewat tautan undangan; kirim ulang undangan atau hapus akun")
)

// GalatValidasi adalah masukan yang salah (400), dengan pesan yang aman ditampilkan ke pengguna.
type GalatValidasi struct{ Pesan string }

func (e *GalatValidasi) Error() string { return e.Pesan }

func galatValidasi(format string, a ...any) error {
	return &GalatValidasi{Pesan: fmt.Sprintf(format, a...)}
}

// UndangInput adalah isian satu akun yang diundang.
type UndangInput struct {
	Nama          string
	Email         string
	Peran         string
	Lembaga       string
	KabupatenKode string
}

// HasilUndangan adalah hasil membuat atau mengirim ulang satu undangan.
type HasilUndangan struct {
	User          *domain.Pengguna
	BerlakuSampai time.Time
	// EmailTerkirim false bila SMTP tidak dikonfigurasi, menolak, atau tidak menjawab dalam BatasWaktuEmail:
	// akunnya tetap ada dan undangan bisa dikirim ulang.
	EmailTerkirim bool
}

// ttlUndangan adalah masa berlaku undangan; konfigurasi nol memakai bawaan.
func (uc *UseCase) ttlUndangan() time.Duration {
	if uc.cfg.UndanganTTL > 0 {
		return uc.cfg.UndanganTTL
	}
	return UndanganTTLBawaan
}

// peranDikenal dan lapisPeran adalah satu-satunya tempat berkas ini bertanya tentang katalog peran.
func lapisPeran(peran string) string { return roleLapis[peran] }

// tokenUndanganBaru membuat token acak 256-bit (base64url tanpa padding, aman di URL) dan hash-nya.
func tokenUndanganBaru() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("buat token undangan: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, sha256Hex(raw), nil
}

// idPenggunaBaru membuat id akun undangan: "USR-" + 16 heksadesimal acak (64 bit, tabrakan diabaikan).
func idPenggunaBaru() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("buat id pengguna: %w", err)
	}
	return "USR-" + strings.ToUpper(hex.EncodeToString(b)), nil
}

// ── Validasi isian ────────────────────────────────────────────────────

// awalanRumus adalah karakter yang membuat spreadsheet membaca sel sebagai rumus. Nilai yang diawalinya
// ditolak saat masuk (nama, email, lembaga) dan, untuk data lama, dinetralkan saat keluar (csvAman).
const awalanRumus = "=+-@"

func diawaliRumus(s string) bool {
	return s != "" && strings.ContainsRune(awalanRumus, rune(s[0]))
}

// adaKarakterKendali melaporkan karakter kendali (termasuk baris baru dan tab) di s.
func adaKarakterKendali(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

// emailValid memeriksa bentuk alamat email biasa: satu @, bagian lokal dan domain tak kosong, domain
// bertitik, tanpa spasi atau karakter pemisah/kutip. Sengaja lebih sempit dari RFC 5322 (tidak ada nama
// tampilan, komentar, atau bagian lokal berkutip): yang diterima aman untuk header email dan sel CSV.
func emailValid(e string) bool {
	if e == "" || len(e) > maksEmail {
		return false
	}
	if strings.ContainsAny(e, " \t\r\n,;:<>()[]\\\"") {
		return false
	}
	lokal, domainEmail, ok := strings.Cut(e, "@")
	if !ok || lokal == "" || domainEmail == "" || strings.Contains(domainEmail, "@") {
		return false
	}
	if len(lokal) > 64 {
		return false
	}
	bagian := strings.Split(domainEmail, ".")
	if len(bagian) < 2 {
		return false
	}
	for _, b := range bagian {
		if b == "" || strings.HasPrefix(b, "-") || strings.HasSuffix(b, "-") {
			return false
		}
	}
	return true
}

// bersihkanAkunUndangan merapikan (spasi tepi, huruf kecil email, huruf besar peran) dan memeriksa isian satu
// akun. Mengembalikan isian yang sudah dirapikan dan daftar pesan galat (kosong bila sah). Keberadaan
// kabupaten dan duplikat email di basis data diperiksa pemanggil (butuh repositori).
func bersihkanAkunUndangan(in UndangInput) (UndangInput, []string) {
	out := UndangInput{
		Nama:          strings.TrimSpace(in.Nama),
		Email:         NormalisasiEmail(in.Email),
		Peran:         strings.ToUpper(strings.TrimSpace(in.Peran)),
		Lembaga:       strings.TrimSpace(in.Lembaga),
		KabupatenKode: strings.TrimSpace(in.KabupatenKode),
	}
	var galat []string

	switch {
	case out.Nama == "":
		galat = append(galat, "Nama wajib diisi.")
	case utf8.RuneCountInString(out.Nama) > maksNama:
		galat = append(galat, fmt.Sprintf("Nama maksimal %d karakter.", maksNama))
	case adaKarakterKendali(out.Nama):
		galat = append(galat, "Nama memuat karakter yang tidak diizinkan.")
	case diawaliRumus(out.Nama):
		galat = append(galat, "Nama tidak boleh diawali = + - atau @ (dibaca sebagai rumus di spreadsheet).")
	}

	switch {
	case out.Email == "":
		galat = append(galat, "Email wajib diisi.")
	case !emailValid(out.Email):
		galat = append(galat, "Email tidak valid.")
	case diawaliRumus(out.Email):
		galat = append(galat, "Email tidak boleh diawali = + - atau @.")
	}

	switch {
	case out.Peran == "":
		galat = append(galat, "Peran wajib diisi.")
	case !peranDikenal(out.Peran):
		galat = append(galat, fmt.Sprintf("Peran tidak dikenal: %s.", out.Peran))
	}

	switch {
	case out.Lembaga == "":
		galat = append(galat, "Lembaga wajib diisi.")
	case utf8.RuneCountInString(out.Lembaga) > maksLembaga:
		galat = append(galat, fmt.Sprintf("Lembaga maksimal %d karakter.", maksLembaga))
	case adaKarakterKendali(out.Lembaga):
		galat = append(galat, "Lembaga memuat karakter yang tidak diizinkan.")
	case diawaliRumus(out.Lembaga):
		galat = append(galat, "Lembaga tidak boleh diawali = + - atau @ (dibaca sebagai rumus di spreadsheet).")
	}

	if out.KabupatenKode != "" && !kodeKabupatenValid(out.KabupatenKode) {
		galat = append(galat, "Kode kabupaten harus 4 digit angka.")
	}
	return out, galat
}

// kodeKabupatenValid: kode BPS kabupaten/kota, empat digit (kolom VARCHAR(4)).
func kodeKabupatenValid(k string) bool {
	if len(k) != 4 {
		return false
	}
	for _, r := range k {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// penggunaUndangan menyusun akun MENUNGGU (tanpa kata sandi) dan undangan pertamanya dari isian yang sudah bersih.
func (uc *UseCase) penggunaUndangan(in UndangInput, aktorID string, sekarang time.Time) (domain.UndanganBaru, string, error) {
	id, err := idPenggunaBaru()
	if err != nil {
		return domain.UndanganBaru{}, "", err
	}
	raw, hash, err := tokenUndanganBaru()
	if err != nil {
		return domain.UndanganBaru{}, "", err
	}
	user := &domain.Pengguna{
		ID: id, Nama: in.Nama, Email: in.Email, Peran: in.Peran, Lapis: lapisPeran(in.Peran), Lembaga: in.Lembaga,
		Status: domain.StatusMenunggu,
	}
	if in.KabupatenKode != "" {
		kode := in.KabupatenKode
		user.KabupatenKode = &kode
	}
	undangan := &domain.Undangan{
		TokenHash: hash, Email: in.Email, ExpiresAt: sekarang.Add(uc.ttlUndangan()),
	}
	if aktorID != "" {
		undangan.DiundangOleh = &aktorID
	}
	return domain.UndanganBaru{User: user, Undangan: undangan}, raw, nil
}

// ── Mengundang ────────────────────────────────────────────────────────

// UndangPengguna membuat akun MENUNGGU dan mengirim email undangan. Akun dan undangannya tersimpan atomik;
// kegagalan mengirim email tidak membatalkannya (HasilUndangan.EmailTerkirim false): admin mengirim ulang.
func (uc *UseCase) UndangPengguna(ctx context.Context, in UndangInput, aktorID string) (*HasilUndangan, error) {
	in, galat := bersihkanAkunUndangan(in)
	if len(galat) > 0 {
		return nil, &GalatValidasi{Pesan: strings.Join(galat, " ")}
	}
	if in.KabupatenKode != "" {
		ada, err := uc.repo.KabupatenAda(ctx, []string{in.KabupatenKode})
		if err != nil {
			return nil, fmt.Errorf("undang pengguna: %w", err)
		}
		if !ada[in.KabupatenKode] {
			return nil, galatValidasi("Kode kabupaten tidak dikenal: %s.", in.KabupatenKode)
		}
	}

	sekarang := jam()
	baru, raw, err := uc.penggunaUndangan(in, aktorID, sekarang)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.BuatUndangan(ctx, []domain.UndanganBaru{baru}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrEmailExists
		}
		return nil, fmt.Errorf("undang pengguna: %w", err)
	}

	terkirim := uc.kirimUndangan(ctx, baru.User, raw)
	uc.writeAudit(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "UNDANG_PENGGUNA", Entitas: "pengguna", EntitasID: baru.User.ID,
		Hasil: audit.HasilBerhasil, Sesudah: userSnapshot(baru.User),
		Metadata: map[string]any{"sumber": "form", "berlaku_sampai": baru.Undangan.ExpiresAt.UTC().Format(time.RFC3339), "email_terkirim": terkirim},
	})
	uc.log.Info("undangan pengguna dibuat", map[string]interface{}{"user_id": baru.User.ID, "email_terkirim": terkirim})
	return &HasilUndangan{User: baru.User, BerlakuSampai: baru.Undangan.ExpiresAt, EmailTerkirim: terkirim}, nil
}

// KirimUlangUndangan mengganti undangan akun MENUNGGU: tautan lama langsung tidak berlaku dan tautan baru
// dikirim ke email akun saat ini.
func (uc *UseCase) KirimUlangUndangan(ctx context.Context, id, aktorID string) (*HasilUndangan, error) {
	raw, hash, err := tokenUndanganBaru()
	if err != nil {
		return nil, err
	}
	sekarang := jam()
	undangan := &domain.Undangan{TokenHash: hash, ExpiresAt: sekarang.Add(uc.ttlUndangan())}
	if aktorID != "" {
		undangan.DiundangOleh = &aktorID
	}
	user, err := uc.repo.GantiUndangan(ctx, id, undangan, sekarang)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, ErrUserNotFound
		case errors.Is(err, domain.ErrBukanMenunggu):
			return nil, ErrBukanMenunggu
		}
		return nil, fmt.Errorf("kirim ulang undangan: %w", err)
	}

	terkirim := uc.kirimUndangan(ctx, user, raw)
	uc.writeAudit(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "UNDANG_ULANG_PENGGUNA", Entitas: "pengguna", EntitasID: user.ID,
		Hasil:    audit.HasilBerhasil,
		Metadata: map[string]any{"berlaku_sampai": undangan.ExpiresAt.UTC().Format(time.RFC3339), "email_terkirim": terkirim},
	})
	uc.log.Info("undangan pengguna dikirim ulang", map[string]interface{}{"user_id": user.ID, "email_terkirim": terkirim})
	return &HasilUndangan{User: user, BerlakuSampai: undangan.ExpiresAt, EmailTerkirim: terkirim}, nil
}

// kirimUndangan mengirim email undangan dan menunggu paling lama BatasWaktuEmail. false bila SMTP tidak
// dikonfigurasi, gagal, atau tidak menjawab. Token tidak pernah masuk log.
func (uc *UseCase) kirimUndangan(ctx context.Context, user *domain.Pengguna, rawToken string) bool {
	if !uc.emailSender.Configured() {
		uc.log.Warn("undangan: SMTP belum dikonfigurasi, email tidak dikirim", map[string]interface{}{"user_id": user.ID})
		return false
	}
	selesai := make(chan error, 1)
	go func() { selesai <- uc.emailSender.SendUndangan(user.Email, user.Nama, rawToken, uc.ttlUndangan()) }()
	select {
	case err := <-selesai:
		if err != nil {
			uc.log.Error("undangan: gagal mengirim email", map[string]interface{}{"user_id": user.ID, "error": err.Error()})
			return false
		}
		return true
	case <-time.After(batasWaktuEmail):
		uc.log.Error("undangan: pengiriman email melewati batas waktu", map[string]interface{}{"user_id": user.ID})
		return false
	case <-ctx.Done():
		uc.log.Warn("undangan: permintaan dibatalkan saat menunggu pengiriman email", map[string]interface{}{"user_id": user.ID})
		return false
	}
}

// ── Aktivasi ──────────────────────────────────────────────────────────

// AktifkanAkun menetapkan kata sandi pertama dari tautan undangan. Token diperiksa dulu (tanpa menghabiskannya),
// lalu kebijakan kata sandi (kata sandi lemah tidak menghanguskan tautan), lalu token dipakai dan akun diaktifkan
// dalam satu transaksi: dua permintaan serentak dengan token yang sama hanya satu yang berhasil.
func (uc *UseCase) AktifkanAkun(ctx context.Context, tokenMentah, password string) error {
	if tokenMentah == "" || len(tokenMentah) > maksTokenKar {
		return ErrUndanganInvalid
	}
	hash := sha256Hex(tokenMentah)
	sekarang := jam()

	tersimpan, err := uc.repo.GetUndangan(ctx, hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUndanganInvalid
		}
		return fmt.Errorf("aktivasi: baca undangan: %w", err)
	}
	if tersimpan.UsedAt != nil || tersimpan.DicabutPada != nil {
		return ErrUndanganInvalid
	}
	if !sekarang.Before(tersimpan.ExpiresAt) {
		return ErrUndanganKedaluwarsa
	}

	if err := ValidatePassword(password); err != nil {
		return err
	}
	if len(password) > maksSandiByte {
		return fmt.Errorf("%w: maksimal %d karakter", ErrWeakPassword, maksSandiByte)
	}

	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("aktivasi: hash kata sandi: %w", err)
	}
	user, err := uc.repo.Aktifkan(ctx, hash, passwordHash, sekarang)
	if err != nil {
		return fmt.Errorf("aktivasi: %w", err)
	}
	if user == nil {
		return ErrUndanganInvalid
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: user.ID, Aksi: "AKTIVASI_AKUN", Entitas: "pengguna", EntitasID: user.ID, Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"status": domain.StatusMenunggu}, Sesudah: map[string]any{"status": user.Status},
	})
	uc.log.Info("akun diaktivasi lewat undangan", map[string]interface{}{"user_id": user.ID})
	return nil
}

// periksaPerubahanStatus menegakkan bahwa status MENUNGGU hanya dikelola alur undangan: akun menunggu tidak
// bisa diaktifkan atau dinonaktifkan dari PATCH (aktivasi hanya lewat tautan undangan; membatalkan undangan
// berarti menghapus akun), dan status MENUNGGU tidak bisa dipasang ke akun yang sudah punya kata sandi.
func periksaPerubahanStatus(user *domain.Pengguna, statusBaru *string) error {
	if statusBaru == nil {
		return nil
	}
	switch {
	case user.Status == domain.StatusMenunggu && *statusBaru != domain.StatusMenunggu:
		return ErrAkunMenunggu
	case user.Status != domain.StatusMenunggu && *statusBaru == domain.StatusMenunggu:
		return galatValidasi("Status menunggu aktivasi hanya diberikan lewat undangan pengguna.")
	}
	return nil
}

// TungguPengirimanEmail menunggu email undangan massal yang masih dikirim di latar belakang (impor CSV).
func (uc *UseCase) TungguPengirimanEmail() { uc.kirimLatar.Wait() }
