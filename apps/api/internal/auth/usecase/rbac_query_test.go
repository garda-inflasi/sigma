package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRolesWithPermission(t *testing.T) {
	assert.Equal(t,
		[]string{"MANAJER_PROGRAM", "MANAJER_DATA", "MANAJER_RISIKO", "OPS_KOORDINATOR"},
		RolesWithPermission("ews.akui"))
	assert.Equal(t, []string{"MANAJER_DATA"}, RolesWithPermission("ews.atur_ambang"))
	assert.Empty(t, RolesWithPermission("izin.tidak.ada"))
}
