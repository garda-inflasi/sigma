package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	c := New(time.Minute)
	assert.NotNil(t, c)
}

func TestSetGet(t *testing.T) {
	c := New(time.Minute)
	c.Set("key1", "value1")

	val, found := c.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "value1", val)
}

func TestGet_NotFound(t *testing.T) {
	c := New(time.Minute)
	_, found := c.Get("nonexistent")
	assert.False(t, found)
}

func TestGet_Expired(t *testing.T) {
	c := New(-time.Second) // already expired
	c.Set("key2", "value2")

	_, found := c.Get("key2")
	assert.False(t, found)
}

func TestDelete(t *testing.T) {
	c := New(time.Minute)
	c.Set("key3", "value3")
	c.Delete("key3")

	_, found := c.Get("key3")
	assert.False(t, found)
}

func TestClear(t *testing.T) {
	c := New(time.Minute)
	c.Set("key4", "value4")
	c.Set("key5", "value5")
	c.Clear()

	_, found := c.Get("key4")
	assert.False(t, found)
	_, found = c.Get("key5")
	assert.False(t, found)
}

func TestSetGet_Int(t *testing.T) {
	c := New(time.Minute)
	c.Set("intkey", 42)

	val, found := c.Get("intkey")
	assert.True(t, found)
	assert.Equal(t, 42, val)
}

func TestSetGet_Struct(t *testing.T) {
	type testStruct struct {
		Name string
	}
	c := New(time.Minute)
	c.Set("structkey", testStruct{Name: "test"})

	val, found := c.Get("structkey")
	assert.True(t, found)
	assert.Equal(t, "test", val.(testStruct).Name)
}

func TestSetWithTTL(t *testing.T) {
	c := New(time.Minute)
	c.SetWithTTL("ttlkey", "ttlvalue", time.Hour)

	val, found := c.Get("ttlkey")
	assert.True(t, found)
	assert.Equal(t, "ttlvalue", val)
}
