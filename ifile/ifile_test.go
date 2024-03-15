package ifile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tomruk/kopyaship/utils"
)

// Test whether it truncates the old ifile contents when appendToExisting is false.
func TestIfileOverwrite(t *testing.T) {
	ifileFormRe := regexp.MustCompile(fmt.Sprintf("^.*%s\n%s\n([/a-zA-Z0-9-_.]+\n)+%s\n$",
		regexp.QuoteMeta(generatedBy),
		regexp.QuoteMeta(beginIndicator),
		regexp.QuoteMeta(endIndicator),
	))

	os.Remove("test_ifile")
	i, err := New("test_ifile", ModeSyncthing, false, utils.NewCLILogger())
	require.NoError(t, err)

	path, err := filepath.Abs("..")
	require.NoError(t, err)
	err = i.Walk(path)
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	content, err := os.ReadFile("test_ifile")
	require.NoError(t, err)

	require.True(t, ifileFormRe.Match(content))

	i, err = New("test_ifile", ModeSyncthing, false, utils.NewCLILogger())
	require.NoError(t, err)
	err = i.Walk(path)
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	require.True(t, ifileFormRe.Match(content))
}

// Test whether it successfully adds entries between beginIndiator and endIndicator, preserving old contents when appendToExisting is true.
func TestIfileAppend(t *testing.T) {
	content := []byte(`# This is a comment.
/this/is/a/test/entry

`)

	ifileFormRe := regexp.MustCompile(fmt.Sprintf("^%s%s\n%s\n([/a-zA-Z0-9-_.]+\n)+%s\n$",
		regexp.QuoteMeta(string(content)),
		regexp.QuoteMeta(generatedBy),
		regexp.QuoteMeta(beginIndicator),
		regexp.QuoteMeta(endIndicator),
	))

	os.Remove("test_ifile")
	err := os.WriteFile("test_ifile", content, 0644)
	require.NoError(t, err)

	i, err := New("test_ifile", ModeSyncthing, true, utils.NewCLILogger())
	require.NoError(t, err)

	path, err := filepath.Abs("..")
	require.NoError(t, err)
	err = i.Walk(path)
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	content, err = os.ReadFile("test_ifile")
	require.NoError(t, err)

	require.True(t, ifileFormRe.Match(content))

	i, err = New("test_ifile", ModeSyncthing, true, utils.NewCLILogger())
	require.NoError(t, err)
	err = i.Walk(path)
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	require.True(t, ifileFormRe.Match(content))
}
