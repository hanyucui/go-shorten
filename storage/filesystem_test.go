package storage_test

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/hanyucui/go-shorten/storage"
)

func setupFilesystemStorage(t testing.TB) storage.NamedStorage {
	dir, err := ioutil.TempDir("", "BenchmarkFilesystem")
	require.Nil(t, err)

	s, err := storage.NewFilesystem(dir)
	require.Nil(t, err)

	return s
}

func TestCleanPath(t *testing.T) {
	testPaths := map[string]string{
		"/asdf":                  "/asdf",
		"/asdf/../../../../path": "/path",
		"../../../../path":       "/path",
	}

	for bad, good := range testPaths {
		assert.Equal(t, storage.CleanPath(bad), good)
	}
}
