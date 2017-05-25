package cmd

import (
	"regexp"
	"strings"

	"github.com/nikkolasg/mulsigo/client"
	"github.com/nikkolasg/mulsigo/slog"
	"github.com/nikkolasg/mulsigo/util"
	"github.com/spf13/cobra"
)

var SecretFile = "secret.toml"
var IdentityFile = "public.toml"

// localCmd represents the local command
var localCmd = &cobra.Command{
	Use:   "local [name]",
	Short: "local generates a local pgp key pair",
	Long:  "local generates a local key pgp key pair using gpg2. This key pair can be used to create a group.toml along other public keys. You can specify an optional name argument if you want a name to be associated with your key pair. If you don't specify a name,a random one will be assigned!",
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			name = util.GetRandomName(0)
		}
		generateKey(name)
	},
}

func init() {
	newCmd.AddCommand(localCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// localCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}

func process(name string) string {
	name = strings.Replace(name, " ", "", -1)
	ok, err := regexp.Match(`^[\w-_]+$`, []byte(name))
	if !ok || err != nil {
		slog.Fatal("name is invalid. Only A-Za-z09_- is allowed.")
	}
	return name
}

func generateKey(name string) {
	name = process(name)
	slog.Debugf("generateKey: name %s", name)
	private, identity := client.NewPrivateIdentity(name)
	slog.Info("generated a new identity with name", name)

	secConfig := rootConfig.Dir(IdentityFolder).Dir(name)
	err := secConfig.Write(SecretFile, private)
	slog.ErrFatal(err)
	slog.Infof("saved private keys in %s", secConfig.RelPath(SecretFile))
	err = secConfig.Write(IdentityFile, identity)
	slog.ErrFatal(err)
	slog.Infof("saved public identity in %s", secConfig.RelPath(IdentityFile))
	slog.Printf("New local identity %s saved at %s. Bye !", name, secConfig.Path())
}
