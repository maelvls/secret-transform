package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getOneOfAnnots(t *testing.T) {
	t.Run("returns first found annotation", func(t *testing.T) {
		annots := map[string]string{
			"foo": "foo-1",
			"bar": "bar-1",
			"baz": "baz-1",
		}
		annot, value := getOneOf(annots, "bar", "foo", "baz")
		assert.Equal(t, "bar", annot)
		assert.Equal(t, "bar-1", value)
	})

	t.Run("returns an empty key when annot not found", func(t *testing.T) {
		annots := map[string]string{
			"foo": "foo-1",
			"bar": "bar-1",
		}
		annot, value := getOneOf(annots, "unknown")
		assert.Equal(t, "", annot)
		assert.Equal(t, "", value)
	})

	t.Run("returns an empty key when annots is nil", func(t *testing.T) {
		annot, value := getOneOf(nil, "foo", "bar")
		assert.Equal(t, "", annot)
		assert.Equal(t, "", value)
	})
}
