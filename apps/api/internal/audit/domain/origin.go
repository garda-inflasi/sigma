package domain

import "context"

// Origin menjelaskan dari mana sebuah permintaan datang; dibawa lewat context
// supaya usecase tidak perlu menerima IP dan user agent sebagai parameter.
type Origin struct {
	IP            string
	UserAgent     string
	CorrelationID string
}

type originKey struct{}

// WithOrigin menyimpan origin pada context.
func WithOrigin(ctx context.Context, origin Origin) context.Context {
	return context.WithValue(ctx, originKey{}, origin)
}

// OriginFrom membaca origin dari context; kosong bila tidak ada.
func OriginFrom(ctx context.Context) Origin {
	origin, _ := ctx.Value(originKey{}).(Origin)
	return origin
}
