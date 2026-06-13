package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) listCmd() *cobra.Command {
	var lang, topic string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Browse Project Gutenberg books with optional filters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("listing books...")
			books, err := a.client.List(cmd.Context(), lang, topic, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(books, len(books))
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "", "filter by ISO 639-1 language code (e.g. en, fr, de)")
	cmd.Flags().StringVar(&topic, "topic", "", "filter by topic (subject or bookshelf)")
	return cmd
}
