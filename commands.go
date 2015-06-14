package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/juju/errors"
)

func setupVendoring() error {
	vendorDirs := []string{".vendor/bin", ".vendor/pkg", ".vendor/src"}

	for _, vendorDir := range vendorDirs {
		err := os.MkdirAll(vendorDir, 0755)

		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func installCommand(c *cli.Context, forceUpdate bool, checkUpstream bool, respectLocked bool) {
	// bunch install
	// bunch install github.com/abc/xyz
	// bunch install github.com/abc/xyz github.com/abc/def
	// bunch install github.com/abc/xyz --save
	// bunch install github.com/abc/xyz -g
	// bunch install abc/xyz # github shorthand

	// bunch update
	// bunch update github.com/abc/xyz
	// bunch update github.com/abc/xyz github.com/abc/def
	// bunch update github.com/abc/xyz --save
	// bunch update github.com/abc/xyz -g

	packages := c.Args()

	err := setupVendoring()
	if err != nil {
		log.Fatalf("unable to set up vendor dirs: %s", err)
	}

	if len(packages) == 0 {
		bunch, err := readBunchfile()
		if err != nil {
			log.Fatalf("unable to read Bunchfile: %s", err)
		}

		err = installPackagesFromBunchfile(bunch, forceUpdate, checkUpstream, respectLocked)

		if err != nil {
			log.Fatalf("failed installing packages: %s %s", err, err.(*errors.Err).StackTrace())
		}
	} else {
		global := c.Bool("g")
		save := c.Bool("save")

		if global && os.Getenv("GOPATH") == "" {
			log.Fatalf("GOPATH must be set when -g used")
		}

		var bunch *BunchFile
		if exists, _ := pathExists("Bunchfile"); exists {
			bunch, err = readBunchfile()
			if err != nil {
				log.Fatalf("unable to read Bunchfile: %s", err)
			}
		} else {
			bunch = createBunchfile()
		}

		err := installPackagesFromRepoStrings(packages, global, forceUpdate, checkUpstream, respectLocked)
		if err != nil {
			log.Fatalf("failed installing packages: %s", err)
		}

		if save {
			for _, pack := range packages {
				err := bunch.AddPackage(pack)

				if err != nil {
					log.Fatalf("failed adding package %s to save list: %s", pack, err)
				}
			}

			err = bunch.Save()
			if err != nil {
				log.Fatalf("failed saving Bunchfile: %s", err)
			}
		}
	}
}

func uninstallCommand(c *cli.Context) {
	// bunch uninstall github.com/abc/xyz
	// bunch uninstall github.com/abc/xyz --save
	// bunch uninstall github.com/abc/xyz -g

	packages := c.Args()

	err := setupVendoring()
	if err != nil {
		log.Fatalf("unable to set up vendor dirs: %s", err)
	}

	if len(packages) == 0 {
		log.Fatalf("uninstall requires an argument")
	} else {
		global := c.Bool("g")
		save := c.Bool("save")

		if global && os.Getenv("GOPATH") == "" {
			log.Fatalf("GOPATH must be set when -g used")
		}

		var bunch *BunchFile
		if exists, _ := pathExists("Bunchfile"); exists {
			bunch, err = readBunchfile()
			if err != nil {
				log.Fatalf("unable to read Bunchfile: %s", err)
			}
		} else {
			bunch = createBunchfile()
		}

		err := removePackages(packages, bunch, global)
		if err != nil {
			log.Fatalf("failed removing packages: %s", err)
		}

		if save {
			for _, pack := range packages {
				err := bunch.RemovePackage(pack)

				if err != nil {
					log.Fatalf("failed removing package %s from save list: %s", pack, err)
				}
			}

			err = bunch.Save()
			if err != nil {
				log.Fatalf("failed saving Bunchfile: %s", err)
			}
		}
	}
}

func pruneCommand(c *cli.Context) {
	// bunch prune

	err := setupVendoring()
	if err != nil {
		log.Fatalf("unable to set up vendor dirs: %s", err)
	}

	var bunch *BunchFile
	if exists, _ := pathExists("Bunchfile"); exists {
		bunch, err = readBunchfile()
		if err != nil {
			log.Fatalf("unable to read Bunchfile: %s", err)
		}
	} else {
		log.Fatalf("can't prune without Bunchfile")
	}

	err = prunePackages(bunch)
	if err != nil {
		log.Fatalf("failed pruning packages: %s", err)
	}
}

func outdatedCommand(c *cli.Context) {
	// bunch outdated

	err := setupVendoring()
	if err != nil {
		log.Fatalf("unable to set up vendor dirs: %s", err)
	}

	var bunch *BunchFile
	if exists, _ := pathExists("Bunchfile"); exists {
		bunch, err = readBunchfile()
		if err != nil {
			log.Fatalf("unable to read Bunchfile: %s", err)
		}
	} else {
		log.Fatalf("can't check for outdated packages without Bunchfile")
	}

	err = checkOutdatedPackages(bunch)
	if err != nil {
		log.Fatalf("failed checking for outdated packages: %s %s", err, err.(*errors.Err).StackTrace())
	}
}

func lockCommand(c *cli.Context) {
	// bunch lock

	err := setupVendoring()
	if err != nil {
		log.Fatalf("unable to set up vendor dirs: %s", err)
	}

	var bunch *BunchFile
	if exists, _ := pathExists("Bunchfile"); exists {
		bunch, err = readBunchfile()
		if err != nil {
			log.Fatalf("unable to read Bunchfile: %s", err)
		}
	} else {
		log.Fatalf("can't lock packages without Bunchfile")
	}

	err = lockPackages(bunch)
	if err != nil {
		log.Fatalf("failed locking packages: %s", err)
	}

}

func generateCommand(c *cli.Context) {
	// bunch generate

	err := setupVendoring()
	if err != nil {
		log.Fatalf("unable to set up vendor dirs: %s", err)
	}

	err = generateBunchfile()
	if err != nil {
		log.Fatalf("failed checking for outdated packages: %s", err)
	}
}

func goCommand(c *cli.Context) {
	// bunch go test
	// bunch go fmt
	// bunch go ...

	err := setVendorEnv()
	if err != nil {
		log.Fatalf("unable to set vendor env: %s", err)
	}

	cmd := exec.Command("go", c.Args()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Fatalf("running 'go %s' failed: %s", strings.Join(c.Args(), " "), err)
	}
}

func execCommand(c *cli.Context) {
	// bunch exec make

	err := setVendorEnv()
	if err != nil {
		log.Fatalf("unable to set vendor env: %s", err)
	}

	cmd := exec.Command(c.Args()[0], c.Args()[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Fatalf("running '%s' failed: %s", strings.Join(c.Args(), " "), err)
	}
}

func shellCommand(c *cli.Context) {
	// bunch shell (bunch exec $SHELL)

	shell := "/bin/bash"
	envShell := os.Getenv("SHELL")
	if envShell != "" {
		shell = envShell
	}

	err := setVendorEnv()
	if err != nil {
		log.Fatalf("unable to set vendor env: %s", err)
	}

	fmt.Printf("starting bunch shell (%s)\n", shell)

	cmd := exec.Command(shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Fatalf("running '%s' failed: %s", shell, err)
	}

	fmt.Println("exiting bunch shell")
}

var shimScript = `#!/bin/bash

PATH=$(echo "$PATH" | sed -e "s|$HOME/.bunch/shims:||g")

if [[ -n $(echo "$PATH" | grep .bunch/shims) ]]; then
  echo bunch warning: unable to remove shim from PATH, falling back to backup PATH
  PATH=/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/bin
fi

if [[ -f "Bunchfile" && -d ".vendor" ]]; then
  WD=$(pwd)
  GOPATH="$WD/.vendor/" GOBIN="$GOPATH/bin" PATH="$GOBIN:$PATH"  go $@
else
  go $@
fi
`

func shimCommand(c *cli.Context) {
	// bunch shim outputs shell script

	err := os.MkdirAll(path.Join(os.Getenv("HOME"), ".bunch", "shims"), 0755)
	if err != nil {
		log.Fatalf("unable to create ~/.bunch")
	}

	goShimDir := path.Join(os.Getenv("HOME"), ".bunch", "shims")
	goShimPath := path.Join(goShimDir, "go")
	err = ioutil.WriteFile(goShimPath, []byte(shimScript), 0755)
	if err != nil {
		log.Fatalf("unable to create shim")
	}

	if len(c.Args()) > 0 {
		fmt.Printf("export PATH=%s:$PATH\n", goShimDir)
	} else {
		fmt.Println(`To have 'go' be automatically bunch-aware, add this to .bash_profile or .zshrc:

if which bunch > /dev/null; then eval "$(bunch shim -)"; fi`)
	}
}
