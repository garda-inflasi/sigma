package notifikasi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/email"
)

// EmailSender adalah pengirim email yang dibutuhkan adaptor (diimplementasi *email.Sender).
type EmailSender interface {
	SendHTML(to, subject, htmlBody string) error
}

// EmailPengirim mengirim notifikasi lewat SMTP.
type EmailPengirim struct {
	sender  EmailSender
	baseURL string
}

// NewEmailPengirim membuat adaptor email. baseURL (mis. https://garda.example.id) dipakai
// untuk tautan ke alert; kosong berarti tautan dihilangkan.
func NewEmailPengirim(sender EmailSender, baseURL string) *EmailPengirim {
	return &EmailPengirim{sender: sender, baseURL: strings.TrimRight(baseURL, "/")}
}

func (*EmailPengirim) Kanal() string { return domain.KanalEmail }

func (e *EmailPengirim) Kirim(ctx context.Context, p domain.Pesan) error {
	if strings.TrimSpace(p.Tujuan) == "" {
		return domain.ErrTanpaTujuan
	}
	subject := fmt.Sprintf("[EWS %s] %s", p.Severity, p.Judul)
	body := fmt.Sprintf(`<p><strong>%s</strong> &mdash; %s</p><p>%s</p>`, html.EscapeString(p.Severity), html.EscapeString(p.Judul), html.EscapeString(p.Isi))
	if e.baseURL != "" {
		link := html.EscapeString(e.baseURL + p.TautanPath)
		body += fmt.Sprintf(`<p><a href="%s">Buka di Garda Inflasi</a></p>`, link)
	}
	// smtp.SendMail tidak menerima ctx; jalankan di goroutine agar pembatalan/timeout tidak menahan pekerja.
	done := make(chan error, 1)
	go func() { done <- e.sender.SendHTML(p.Tujuan, subject, body) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("kirim email: %w", ctx.Err())
	}
}

// VAPID adalah kunci identitas server untuk Web Push.
type VAPID struct {
	PublicKey, PrivateKey string
	// Subject adalah kontak operator, mis. mailto:ews@garda.example.id.
	Subject string
}

// Configured melaporkan apakah kedua kunci terisi.
func (v VAPID) Configured() bool { return v.PublicKey != "" && v.PrivateKey != "" }

// VAPIDFromEnv membaca VAPID_PUBLIC_KEY, VAPID_PRIVATE_KEY, dan VAPID_SUBJECT.
func VAPIDFromEnv() VAPID {
	return VAPID{
		PublicKey: os.Getenv("VAPID_PUBLIC_KEY"), PrivateKey: os.Getenv("VAPID_PRIVATE_KEY"), Subject: os.Getenv("VAPID_SUBJECT"),
	}
}

// LanggananStore adalah akses data langganan yang dibutuhkan adaptor push.
type LanggananStore interface {
	ListLangganan(ctx context.Context, penggunaID string) ([]domain.LanggananPush, error)
	HapusLanggananKadaluwarsa(ctx context.Context, endpoint string) error
}

// kirimPush mengirim satu payload ke satu langganan (diganti di tes).
type kirimPush func(ctx context.Context, payload []byte, sub *webpush.Subscription, opts *webpush.Options) (*http.Response, error)

// PushPengirim mengirim Web Push ke semua perangkat terdaftar pengguna.
type PushPengirim struct {
	store LanggananStore
	vapid VAPID
	kirim kirimPush
}

// NewPushPengirim membuat adaptor Web Push (PWA).
func NewPushPengirim(store LanggananStore, vapid VAPID) *PushPengirim {
	return &PushPengirim{store: store, vapid: vapid, kirim: webpush.SendNotificationWithContext}
}

func (*PushPengirim) Kanal() string { return domain.KanalPush }

// Kirim sukses bila minimal satu perangkat menerima. Langganan yang ditolak layanan push
// (404/410) dibuang. Tanpa langganan yang tersisa hasilnya domain.ErrTanpaTujuan.
func (p *PushPengirim) Kirim(ctx context.Context, m domain.Pesan) error {
	subs, err := p.store.ListLangganan(ctx, m.PenggunaID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"title": fmt.Sprintf("EWS %s: %s", m.Severity, m.Judul), "body": m.Isi, "url": m.TautanPath, "tag": m.AlertID,
	})
	if err != nil {
		return fmt.Errorf("encode push: %w", err)
	}

	var sukses int
	var galat []error
	for _, l := range subs {
		resp, err := p.kirim(ctx, payload, &webpush.Subscription{Endpoint: l.Endpoint, Keys: webpush.Keys{P256dh: l.P256dh, Auth: l.Auth}}, &webpush.Options{
			Subscriber: p.vapid.Subject, VAPIDPublicKey: p.vapid.PublicKey, VAPIDPrivateKey: p.vapid.PrivateKey, TTL: 3600, Urgency: webpush.UrgencyHigh,
		})
		if err != nil {
			galat = append(galat, err)
			continue
		}
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			if err := p.store.HapusLanggananKadaluwarsa(ctx, l.Endpoint); err != nil {
				galat = append(galat, err)
			}
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			sukses++
		default:
			galat = append(galat, fmt.Errorf("layanan push menjawab %d", resp.StatusCode))
		}
	}
	switch {
	case sukses > 0:
		return nil
	case len(galat) > 0:
		return errors.Join(galat...)
	default:
		return fmt.Errorf("%w: pengguna belum memiliki langganan push aktif", domain.ErrTanpaTujuan)
	}
}

// DariEnv merakit adaptor yang dikonfigurasi lewat env: email bila SMTP_HOST terisi,
// push bila kunci VAPID terisi. Tanpa konfigurasi, tidak ada adaptor (outbox saja).
func DariEnv(store LanggananStore) []domain.Pengirim {
	var out []domain.Pengirim
	if sender := email.New(email.ConfigFromEnv()); sender.Configured() {
		out = append(out, NewEmailPengirim(sender, os.Getenv("APP_PUBLIC_URL")))
	}
	if vapid := VAPIDFromEnv(); vapid.Configured() {
		out = append(out, NewPushPengirim(store, vapid))
	}
	return out
}
