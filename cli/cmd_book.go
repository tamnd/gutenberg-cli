package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func (a *App) bookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "book <id>",
		Short: "Fetch a single Project Gutenberg book by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return codeError(exitUsage, fmt.Errorf("id must be a positive integer, got %q", args[0]))
			}
			book, err := a.client.GetBook(cmd.Context(), id)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.render(book)
		},
	}
	return cmd
}
