package stores

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/xerrors"
)

func cp(from, to string) error {
	from, err := homedir.Expand(from)
	if err != nil {
		return xerrors.Errorf("cp: expanding from: %w", err)
	}

	to, err = homedir.Expand(to)
	if err != nil {
		return xerrors.Errorf("cp: expanding to: %w", err)
	}

	if filepath.Base(from) != filepath.Base(to) {
		return xerrors.Errorf("cp: base names must match ('%s' != '%s')", filepath.Base(from), filepath.Base(to))
	}

	toDir := filepath.Dir(to)

	log.Debugw("cp sector data", "from", from, "to", toDir)

	var errOut bytes.Buffer

	cmd := exec.Command("/usr/bin/env", "cp", "-r", from, toDir)
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return xerrors.Errorf("exec cp (stderr: %s): %w", strings.TrimSpace(errOut.String()), err)
	}

	return nil
}
