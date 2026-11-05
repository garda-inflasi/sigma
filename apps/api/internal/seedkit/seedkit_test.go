package seedkit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func noop(context.Context, *gorm.DB, time.Time) (Result, error) { return Result{}, nil }

func seeder(name string, tier Tier, dependsOn, after []string) Seeder {
	return Seeder{Name: name, Tier: tier, DependsOn: dependsOn, After: after, Run: noop}
}

func names(plan []Seeder) []string {
	out := make([]string, len(plan))
	for i, s := range plan {
		out[i] = s.Name
	}
	return out
}

func registryOf(t *testing.T, seeders ...Seeder) *Registry {
	t.Helper()
	r := NewRegistry()
	for _, s := range seeders {
		require.NoError(t, r.Register(s))
	}
	return r
}

func TestRegister_RejectsInvalidSeeders(t *testing.T) {
	cases := map[string]Seeder{
		"tanpa nama":               {Tier: TierReference, Run: noop},
		"lapis tidak dikenal":      {Name: "x.test", Tier: "test", Run: noop},
		"nama tidak cocok lapis":   {Name: "x.demo", Tier: TierReference, Run: noop},
		"tanpa fungsi Run":         {Name: "x.reference", Tier: TierReference},
		"nama tanpa titik lapis":   {Name: "x", Tier: TierReference, Run: noop},
		"lapis hanya sebagai nama": {Name: "reference", Tier: TierReference, Run: noop},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, NewRegistry().Register(s))
		})
	}
}

func TestRegister_RejectsDuplicateName(t *testing.T) {
	r := registryOf(t, seeder("x.reference", TierReference, nil, nil))
	assert.ErrorContains(t, r.Register(seeder("x.reference", TierReference, nil, nil)), "dua kali")
}

func TestPlan_OrdersByDependenciesThenTierThenName(t *testing.T) {
	r := registryOf(t,
		seeder("b.demo", TierDemo, []string{"b.reference"}, nil),
		seeder("b.reference", TierReference, nil, nil),
		seeder("a.demo", TierDemo, nil, nil),
		seeder("a.reference", TierReference, nil, []string{"b.reference"}),
		seeder("c.admin", TierAdmin, nil, nil),
	)

	plan, err := r.Plan(AllTiers...)

	require.NoError(t, err)
	assert.Equal(t, []string{"b.reference", "a.reference", "c.admin", "a.demo", "b.demo"}, names(plan))
}

func TestPlan_FiltersByTierAndTreatsUnselectedDependencyAsDone(t *testing.T) {
	r := registryOf(t,
		seeder("m.reference", TierReference, nil, nil),
		seeder("m.demo", TierDemo, []string{"m.reference"}, nil),
	)

	plan, err := r.Plan(TierDemo)

	require.NoError(t, err)
	assert.Equal(t, []string{"m.demo"}, names(plan))
}

func TestPlan_SoftDependencyIgnoredWhenNotRegistered(t *testing.T) {
	r := registryOf(t, seeder("m.reference", TierReference, nil, []string{"master.reference"}))

	plan, err := r.Plan(TierReference)

	require.NoError(t, err)
	assert.Equal(t, []string{"m.reference"}, names(plan))
}

func TestPlan_HardDependencyMustBeRegistered(t *testing.T) {
	r := registryOf(t, seeder("m.demo", TierDemo, []string{"hilang.reference"}, nil))

	_, err := r.Plan(TierReference)

	require.Error(t, err, "galat berlaku walau lapis seeder itu tidak dipilih")
	assert.ErrorContains(t, err, "m.demo bergantung pada hilang.reference")
}

func TestPlan_DetectsCycle(t *testing.T) {
	r := registryOf(t,
		seeder("a.reference", TierReference, []string{"b.reference"}, nil),
		seeder("b.reference", TierReference, []string{"a.reference"}, nil),
	)

	_, err := r.Plan(TierReference)

	assert.ErrorContains(t, err, "berputar")
}

func TestParseTiers(t *testing.T) {
	cases := map[string][]Tier{
		"all":              {TierReference, TierAdmin, TierDemo},
		"demo":             {TierDemo},
		"reference admin":  {TierReference, TierAdmin},
		"admin admin demo": {TierAdmin, TierDemo},
		"demo all":         {TierReference, TierAdmin, TierDemo},
	}
	for input, want := range cases {
		got, err := ParseTiers(strings.Fields(input))
		require.NoError(t, err, input)
		assert.Equal(t, want, got, input)
	}

	_, err := ParseTiers([]string{"reference", "test"})
	assert.ErrorContains(t, err, "tidak dikenal")
	_, err = ParseTiers(nil)
	assert.ErrorContains(t, err, "minimal satu lapis")
}

func TestRun_RefusesDemoInProductionBeforeRunningAnything(t *testing.T) {
	ran := false
	spy := func(context.Context, *gorm.DB, time.Time) (Result, error) { ran = true; return nil, nil }
	plan := []Seeder{
		{Name: "m.reference", Tier: TierReference, Run: spy},
		{Name: "m.demo", Tier: TierDemo, Run: spy},
	}

	for _, env := range []string{"production", "Production", " production "} {
		_, err := Run(t.Context(), nil, plan, time.Now(), env)

		require.ErrorIs(t, err, ErrDemoInProduction, env)
		assert.ErrorContains(t, err, "m.demo")
	}
	assert.False(t, ran, "tidak boleh ada seeder yang jalan bila rencana memuat demo di produksi")
}

func TestRun_AllowsReferenceAndAdminInProduction(t *testing.T) {
	plan := []Seeder{
		seeder("m.reference", TierReference, nil, nil),
		seeder("m.admin", TierAdmin, nil, nil),
	}

	summary, err := Run(t.Context(), nil, plan, time.Now(), "production")

	require.NoError(t, err)
	assert.Len(t, summary, 2)
}

func TestRun_StopsAtFirstErrorAndNamesTheSeeder(t *testing.T) {
	boom := errors.New("meledak")
	plan := []Seeder{
		seeder("a.reference", TierReference, nil, nil),
		{Name: "b.reference", Tier: TierReference, Run: func(context.Context, *gorm.DB, time.Time) (Result, error) { return nil, boom }},
		seeder("c.reference", TierReference, nil, nil),
	}

	summary, err := Run(t.Context(), nil, plan, time.Now(), "development")

	require.ErrorIs(t, err, boom)
	assert.ErrorContains(t, err, "b.reference")
	assert.Len(t, summary, 1, "hanya seeder yang sukses sebelum galat yang tercatat")
}

func TestRun_PassesNowToSeeders(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	var got time.Time
	plan := []Seeder{{Name: "m.demo", Tier: TierDemo, Run: func(_ context.Context, _ *gorm.DB, n time.Time) (Result, error) {
		got = n
		return Result{"lot": 3}, nil
	}}}

	summary, err := Run(t.Context(), nil, plan, now, "development")

	require.NoError(t, err)
	assert.Equal(t, now, got)
	assert.Equal(t, Result{"lot": 3}, summary[0].Result)
}

func TestResult_StringIsSortedByLabel(t *testing.T) {
	assert.Equal(t, "fasilitas=5 stok=23", Result{"stok": 23, "fasilitas": 5}.String())
	assert.Empty(t, Result(nil).String())
}
