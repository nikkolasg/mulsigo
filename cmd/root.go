package cmd

import (
	"fmt"
	"os"

	"github.com/nikkolasg/mulsigo/slog"
	"github.com/nikkolasg/mulsigo/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AppName = "mulsigo"
var Version = "v0.01+alpha"

var rootConfig *util.Config

var verbose bool
var debug bool

// can be overwritten by flags
var homedir = util.DefaultConfigPath(AppName)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "mulsigo",
	Short: "multisignature made easy",
	Long: `mulsigo is a tool that enables a group of people to sign 
in a decentralized manner. Mulsigo  can create a decentralized public key,
with each participants receiving a share of the private key. The private key 
is never computed locally but generated in a decentralized fashion.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			slog.Level = slog.LevelInfo
		}
		if debug {
			slog.Level = slog.LevelDebug
		}
		banner()
		rootConfig = util.NewConfigWithPath(homedir)
		slog.Info("configuration folder:", rootConfig.Path())

	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//Run: func(cmd *cobra.Command, args []string) {
	//},
}

func banner() {
	slog.Info(AppName, Version, "- by nikkolasg")
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose mode")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
	RootCmd.PersistentFlags().StringVarP(&homedir, "homedir", "d", util.DefaultConfigPath(AppName), "home directory where mulsigo stores the keying material")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	viper.SetConfigName(".mulsigo") // name of config file (without extension)
	viper.AddConfigPath("$HOME")    // adding home directory as first search path
	viper.AutomaticEnv()            // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
