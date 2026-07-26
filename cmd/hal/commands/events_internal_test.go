package commands

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCompileGlobs(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice for no patterns", func(t *testing.T) {
		t.Parallel()

		res, err := compileGlobs(nil)
		assert.NilError(t, err)
		assert.Equal(t, len(res), 0)
	})

	t.Run("substring match without wildcards", func(t *testing.T) {
		t.Parallel()

		res, err := compileGlobs([]string{"camera"})
		assert.NilError(t, err)
		assert.Equal(t, len(res), 1)
		assert.Assert(t, res[0].MatchString(`{"entity_id":"camera.front"}`))
		assert.Assert(t, !res[0].MatchString(`{"entity_id":"light.kitchen"}`))
	})

	t.Run("star wildcard matches across characters", func(t *testing.T) {
		t.Parallel()

		res, err := compileGlobs([]string{"*sun.sun*"})
		assert.NilError(t, err)
		assert.Assert(t, res[0].MatchString(`{"entity_id":"sun.sun","state":"above_horizon"}`))
		assert.Assert(t, !res[0].MatchString(`{"entity_id":"moon.moon"}`))
	})

	t.Run("question mark matches single character", func(t *testing.T) {
		t.Parallel()

		res, err := compileGlobs([]string{"a?c"})
		assert.NilError(t, err)
		assert.Assert(t, res[0].MatchString("abc"))
		assert.Assert(t, res[0].MatchString("axc"))
		assert.Assert(t, !res[0].MatchString("ac"))
	})

	t.Run("matches across newlines", func(t *testing.T) {
		t.Parallel()

		res, err := compileGlobs([]string{"foo*bar"})
		assert.NilError(t, err)
		assert.Assert(t, res[0].MatchString("foo\nbar"))
	})

	t.Run("compiles multiple patterns", func(t *testing.T) {
		t.Parallel()

		res, err := compileGlobs([]string{"foo", "bar"})
		assert.NilError(t, err)
		assert.Equal(t, len(res), 2)
	})
}
