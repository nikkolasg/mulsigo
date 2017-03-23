package cmd

import (
	"fmt"

	"github.com/nikkolasg/mulsigo/client"
	"github.com/nikkolasg/mulsigo/slog"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list all distributed keys on this system. Think gpg --list-keys",
	Long: `list all local and distributed keys stored on this system. It looks for default 
	path configuration, but you can use -p flag to provide custom paths. It shows
	the gpg id, name and email for each keys.`,

	Run: func(cmd *cobra.Command, args []string) {
		local, err := cmd.Flags().GetBool("local")
		slog.ErrFatal(err)
		dist, err := cmd.Flags().GetBool("dist")
		slog.ErrFatal(err)

		if local && dist {
			slog.Fatal("can't list only local *and* dist")
		}

		if local {
			listLocalKeys()
		} else if dist {
			listDistKeys()
		} else {
			listLocalKeys()
			listDistKeys()
		}
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolP("local", "l", false, "list only local identities")
	listCmd.Flags().BoolP("dist", "d", false, "list only registered distributed keys")
}

func listLocalKeys() {
	secConfig := rootConfig.Dir(IdentityFolder)
	slog.Print("Local identities:")
	for _, dir := range secConfig.ListDirs() {
		c := secConfig.Dir(dir)
		p := &client.Private{}
		slog.ErrFatal(c.Read(SecretFile, p))
		id := &client.Identity{}
		slog.ErrFatal(c.Read(IdentityFile, id))
		// simple stupid test to check if they're correlated
		if !client.Suite.Point().Mul(nil, p.Key).Equal(id.Public) {
			slog.Fatal("CORRUPTED key folder: " + dir)
		}
		fmt.Println(id.Repr() + "\n")
	}
}

func listDistKeys() {

}
