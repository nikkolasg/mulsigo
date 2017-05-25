package cmd

import (
	"os"
	"testing"

	"github.com/nikkolasg/mulsigo/slog"
	"github.com/stretchr/testify/assert"
)

// a dummy test to see how to test command line application
func TestList(t *testing.T) {

	args := os.Args
	os.Args = append(os.Args, "list")

	//buff := new(bytes.Buffer)
	//slog.Output = buff
	err := RootCmd.Execute()
	assert.Nil(t, err)
	//assert.Contains(t, buff.String(), "no local keys")
	os.Args = args
	slog.Output = os.Stdout
}
