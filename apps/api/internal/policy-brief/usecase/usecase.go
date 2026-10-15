package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.PolicyBrief, error) {
	return uc.repo.List(ctx)
}

func (uc *UseCase) Generate(ctx context.Context, periode string) (*domain.PolicyBrief, error) {
	pb := &domain.PolicyBrief{
		Judul:       fmt.Sprintf("Policy Brief - %s", periode),
		Periode:     periode,
		Ringkasan:   "Policy brief sedang diproses.",
		Status:      "DRAFT",
		DibuatPada:  time.Now(),
		DiupdatePada: time.Now(),
	}

	if err := uc.repo.Create(ctx, pb); err != nil {
		return nil, err
	}

	return pb, nil
}
