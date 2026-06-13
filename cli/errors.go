package cli

import (
	"errors"

	"github.com/tamnd/gutenberg-cli/gutenberg"
)

func isNotFound(err error) bool {
	return errors.Is(err, gutenberg.ErrNotFound)
}
