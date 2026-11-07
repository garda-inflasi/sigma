package schematest

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// column adalah satu baris information_schema.columns.
type column struct {
	TableName string `gorm:"column:table_name"`
	Name      string `gorm:"column:name"`
	UDTName   string `gorm:"column:udt_name"`
	DataType  string `gorm:"column:data_type"`
}

// dbSchema memetakan tabel -> nama kolom -> kolom.
type dbSchema map[string]map[string]column

const usingEnumDataType = "USER-DEFINED"

func loadSchema(t *testing.T, db *gorm.DB) dbSchema {
	t.Helper()
	var rows []column
	require.NoError(t, db.Raw(`
		SELECT table_name  AS table_name,
		       column_name AS name,
		       udt_name    AS udt_name,
		       data_type   AS data_type
		  FROM information_schema.columns
		 WHERE table_schema = 'public'`).Scan(&rows).Error)

	out := dbSchema{}
	for _, r := range rows {
		if out[r.TableName] == nil {
			out[r.TableName] = map[string]column{}
		}
		out[r.TableName][r.Name] = r
	}
	return out
}

// compareEntities mengembalikan ketidakcocokan antara entity dan skema aktual:
// tabel hilang, kolom hilang, dan tipe tak kompatibel.
func compareEntities(entities []any, actual dbSchema) []string {
	var problems []string
	for _, entity := range entities {
		problems = append(problems, compareEntity(entity, actual)...)
	}
	sort.Strings(problems)
	return problems
}

func compareEntity(entity any, actual dbSchema) []string {
	name := reflect.TypeOf(entity).String()
	if _, ok := entity.(schema.Tabler); !ok {
		return []string{fmt.Sprintf("%s: tidak punya TableName()", name)}
	}
	parsed, err := schema.Parse(entity, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return []string{fmt.Sprintf("%s: gagal membaca tag GORM: %v", name, err)}
	}

	columns, ok := actual[parsed.Table]
	if !ok {
		return []string{fmt.Sprintf("%s: tabel %q tidak ada di migrasi", name, parsed.Table)}
	}

	var problems []string
	for _, field := range parsed.Fields {
		if field.DBName == "" {
			continue // field tak dipetakan (gorm:"-")
		}
		col, ok := columns[field.DBName]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: kolom %s.%s tidak ada di migrasi", name, parsed.Table, field.DBName))
		case !typeCompatible(field.DataType, col):
			problems = append(problems, fmt.Sprintf("%s: kolom %s.%s bertipe %s di migrasi, entity memakai %s (%s)",
				name, parsed.Table, field.DBName, col.UDTName, field.DataType, field.FieldType))
		case col.UDTName == "uuid" && !strings.EqualFold(field.TagSettings["TYPE"], "uuid"):
			// docs/SKEMA-DATABASE.md §4: kolom uuid harus dideklarasikan type:uuid supaya GORM dan pembaca kode
			// tahu nilainya UUID (bukan teks bebas) dan AutoMigrate/tes tidak membuat varchar.
			problems = append(problems, fmt.Sprintf("%s: kolom %s.%s bertipe uuid di migrasi; tag GORM harus memuat type:uuid",
				name, parsed.Table, field.DBName))
		}
	}
	return problems
}

// pgTypesByGormType memetakan tipe data GORM ke udt_name PostgreSQL yang kompatibel.
var pgTypesByGormType = map[schema.DataType][]string{
	schema.Bool:  {"bool"},
	schema.Int:   {"int2", "int4", "int8"},
	schema.Uint:  {"int2", "int4", "int8"},
	schema.Float: {"float4", "float8", "numeric"},
	// json/jsonb: string membawa dokumen JSON mentah.
	schema.String: {"varchar", "text", "bpchar", "citext", "uuid", "json", "jsonb"},
	schema.Time:   {"timestamp", "timestamptz", "date"},
	schema.Bytes:  {"bytea"},
}

// customTypeAliases menormalkan tipe yang ditulis di tag `type:` ke udt_name.
var customTypeAliases = map[string]string{
	"decimal":          "numeric",
	"integer":          "int4",
	"bigint":           "int8",
	"smallint":         "int2",
	"boolean":          "bool",
	"double precision": "float8",
}

func typeCompatible(gormType schema.DataType, col column) bool {
	if accepted, ok := pgTypesByGormType[gormType]; ok {
		if gormType == schema.String && col.DataType == usingEnumDataType {
			return true // enum PostgreSQL dibaca sebagai string
		}
		return slices.Contains(accepted, col.UDTName)
	}
	return normalizeCustomType(string(gormType)) == col.UDTName
}

// normalizeCustomType mengubah "numeric(18,2)" atau "JSONB" menjadi udt_name.
func normalizeCustomType(declared string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(declared)), "(")
	base = strings.TrimSpace(base)
	if alias, ok := customTypeAliases[base]; ok {
		return alias
	}
	return base
}
