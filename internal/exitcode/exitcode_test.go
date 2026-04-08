package exitcode

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrap(t *testing.T) {
	t.Run("wraps error with code", func(t *testing.T) {
		inner := errors.New("bad input")
		err := Wrap(inner, Usage)

		var exitErr *Error
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, Usage, exitErr.Code)
		assert.Equal(t, "bad input", exitErr.Error())
	})

	t.Run("nil returns nil", func(t *testing.T) {
		assert.NoError(t, Wrap(nil, Usage))
	})

	t.Run("unwrap preserves inner error", func(t *testing.T) {
		inner := errors.New("original")
		err := Wrap(inner, Auth)
		require.True(t, errors.Is(err, inner))
	})
}

func TestOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, 0},
		{"unwrapped error", errors.New("generic"), 1},
		{"usage error", Wrap(errors.New("bad flag"), Usage), Usage},
		{"auth error", Wrap(errors.New("no creds"), Auth), Auth},
		{"unavailable error", Wrap(errors.New("timeout"), Unavailable), Unavailable},
		{"wrapped exit error", fmt.Errorf("context: %w", Wrap(errors.New("inner"), Auth)), Auth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Of(tt.err))
		})
	}
}
