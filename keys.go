package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"golang.org/x/crypto/openpgp/packet"
)

//var defaultTmpDir = "/tmp"
//var defaultTmpDir = os.TempDir()
var defaultTmpDir string
var createKeyArgs []string
var readPrivateKeyArgs []string

func init() {
	dir, _ := os.Getwd()
	defaultTmpDir = path.Join(dir, "test")
	createKeyArgs = []string{"--homedir", defaultTmpDir, "--expert", "--full-gen-key"}
	readPrivateKeyArgs = []string{"--homedir", defaultTmpDir, "--export-secret-keys", "--homedir", defaultTmpDir}
}

var typeKey = "10"
var typeCurve = "1"
var defaultValidity = "1y"

func CreateEd25519Key(batch string) {
	verifyTmpDir()

	var fname = path.Join(defaultTmpDir, "batch")
	if err := ioutil.WriteFile(fname, []byte(batch), 0777); err != nil {
		Fatal("could not write batch file: " + err.Error())
	}
	defer os.Remove(fname)

	if batch != "" {
		createKeyArgs = append(createKeyArgs, "--batch", fname)
		fmt.Println("overriding input")
	}
	cmd := exec.Command("gpg", createKeyArgs...)

	if err := cmd.Run(); err != nil {
		Fatal("error creating key with gpg")
	}
}

func ReadEd25519Key() (*packet.PrivateKey, error) {
	var b bytes.Buffer
	cmd := exec.Command("gpg", readPrivateKeyArgs...)
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	p, err := packet.Read(&b)
	if err != nil {
		return nil, err
	}

	privKey, ok := p.(*packet.PrivateKey)
	if !ok {
		return nil, errors.New("it's not a private key")
	}
	return privKey, nil
}

// returns false if the directory was not present
func verifyTmpDir() bool {
	var ret = true
	if exists(defaultTmpDir) == false {
		os.Mkdir(defaultTmpDir, 0777)
		ret = false
	}
	removeContents(defaultTmpDir)
	return ret
}

// taken from https://stackoverflow.com/questions/33450980/golang-remove-all-contents-of-a-directory
func removeContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}
