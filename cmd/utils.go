package cmd

import (
	"fmt"
	"os"
	"os/user"

	"github.com/pkg/errors"
)

func checkIfRoot() (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, errors.Wrap(err, "get current user")
	}
	return currentUser.Username == "root", nil
}

func doRootCheck() {
	ok, err := checkIfRoot()
	if err != nil {
		fmt.Printf("Failed to check whether the command is ran by root: %v.\n", err)
		os.Exit(1)
	}

	if !ok {
		fmt.Printf("Root permissions are required.\n")
		os.Exit(1)
	}
}
