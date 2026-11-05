package schematest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type sampleOK struct {
	ID       string    `gorm:"column:id;primaryKey"`
	Jumlah   int       `gorm:"column:jumlah"`
	Harga    float64   `gorm:"column:harga;type:numeric(18,2)"`
	Level    string    `gorm:"column:level"`
	Dokumen  *string   `gorm:"column:dokumen"`
	Aktif    bool      `gorm:"column:aktif"`
	Dibuat   time.Time `gorm:"column:dibuat"`
	Turunan  string    `gorm:"-"`
	Metadata string    `gorm:"column:metadata;type:jsonb"`
}

func (sampleOK) TableName() string { return "sampel" }

type sampleBad struct {
	ID     int64  `gorm:"column:id;primaryKey"`
	Hilang string `gorm:"column:hilang"`
}

func (sampleBad) TableName() string { return "sampel" }

type sampleNoTable struct {
	ID string `gorm:"column:id"`
}

func (sampleNoTable) TableName() string { return "tak_ada" }

type sampleNoTableName struct {
	ID string `gorm:"column:id"`
}

func samplesSchema() dbSchema {
	col := func(name, udt, dataType string) column {
		return column{TableName: "sampel", Name: name, UDTName: udt, DataType: dataType}
	}
	return dbSchema{"sampel": {
		"id":       col("id", "varchar", "character varying"),
		"jumlah":   col("jumlah", "int4", "integer"),
		"harga":    col("harga", "numeric", "numeric"),
		"level":    col("level", "level_ews", usingEnumDataType),
		"dokumen":  col("dokumen", "jsonb", "jsonb"),
		"aktif":    col("aktif", "bool", "boolean"),
		"dibuat":   col("dibuat", "timestamptz", "timestamp with time zone"),
		"metadata": col("metadata", "jsonb", "jsonb"),
	}}
}

func TestCompareEntities_AcceptsCompatibleTypes(t *testing.T) {
	assert.Empty(t, compareEntities([]any{&sampleOK{}}, samplesSchema()))
}

func TestCompareEntities_ReportsEveryKindOfMismatch(t *testing.T) {
	problems := compareEntities([]any{&sampleBad{}, &sampleNoTable{}, &sampleNoTableName{}}, samplesSchema())

	assert.ElementsMatch(t, []string{
		"*schematest.sampleBad: kolom sampel.id bertipe varchar di migrasi, entity memakai int (int64)",
		"*schematest.sampleBad: kolom sampel.hilang tidak ada di migrasi",
		`*schematest.sampleNoTable: tabel "tak_ada" tidak ada di migrasi`,
		"*schematest.sampleNoTableName: tidak punya TableName()",
	}, problems)
}
