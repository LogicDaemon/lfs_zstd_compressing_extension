// lfs-zstd-filter is a git-lfs extension that transparently compresses files
// with zstd before they are stored in LFS object storage, and decompresses
// them on checkout.
//
// It is designed to be registered as an LFS extension in git config:
//
//	[lfs "extension.zstd"]
//	  clean  = lfs-zstd-filter clean %f
//	  smudge = lfs-zstd-filter smudge %f
//	  priority = 0
//
// The binary is a pure stdin→stdout transformer; all pointer management,
// OID computation, local object storage, and server communication remain
// with git-lfs. Stock LFS servers see ordinary LFS objects whose content
// happens to be a zstd-compressed stream.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/LogicDaemon/lfs_zstd_compressing_extension/internal/compress"
)

const usage = `lfs-zstd-filter <command> [args...]

Commands:
  clean   <file>      Read raw bytes from stdin, write zstd-compressed bytes to stdout.
                      Every file is compressed unconditionally, including .zst files.
  smudge  <file>      Read zstd-compressed bytes from stdin, write original bytes to stdout.
                      Non-zstd input is passed through unchanged (migration path for objects
                      committed before the extension was enabled).
  install [--global]  Register the extension in git config. Defaults to local (per-repo).
                      Use --global to install for all repositories for the current user.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]

	var err error
	switch cmd {
	case "clean":
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, "lfs-zstd-filter: clean requires a filename argument\n")
			os.Exit(1)
		}
		err = clean(os.Stdin, os.Stdout)
	case "smudge":
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, "lfs-zstd-filter: smudge requires a filename argument\n")
			os.Exit(1)
		}
		err = smudge(os.Stdin, os.Stdout)
	case "install":
		err = installCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "lfs-zstd-filter: unknown command %q\n\n%s", cmd, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "lfs-zstd-filter %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

// clean compresses src into dst using zstd, unconditionally.
// Every file — including already-compressed formats such as .zst, .gz, .jpg —
// is wrapped in a new zstd frame. zstd will store incompressible data with
// minimal overhead rather than expanding it significantly.
func clean(src io.Reader, dst io.Writer) error {
	return compress.Compress(dst, src)
}

// smudge decompresses a zstd stream from src into dst. If the input does not
// start with our custom 12-byte LFSZ skippable frame, it is copied through
// unchanged. This is the migration path for LFS objects that were stored
// before the extension was enabled; those objects contain raw bytes (or standard
// zstd files) and must be served as-is to avoid corrupting user working trees.
func smudge(src io.Reader, dst io.Writer) error {
	// Peek at the first 12 bytes to decide whether decompression is needed.
	header := make([]byte, 12)
	n, err := io.ReadFull(src, header)
	header = header[:n]

	full := io.MultiReader(bytes.NewReader(header), src)

	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return fmt.Errorf("reading input: %w", err)
	}

	if !compress.IsLFSZstd(header) {
		// Not our custom LFSZ skippable frame — pre-extension object; copy through unchanged.
		_, err = io.Copy(dst, full)
		return err
	}

	return compress.Decompress(dst, full)
}

func installCmd(args []string) error {
	global := false
	if len(args) > 0 && args[0] == "--global" {
		global = true
	}

	configs := []struct{ key, val string }{
		{"lfs.extension.zstd.clean", "lfs-zstd-filter clean %f"},
		{"lfs.extension.zstd.smudge", "lfs-zstd-filter smudge %f"},
		{"lfs.extension.zstd.priority", "0"},
	}

	for _, cfg := range configs {
		gitArgs := []string{"config"}
		if global {
			gitArgs = append(gitArgs, "--global")
		} else {
			gitArgs = append(gitArgs, "--local")
		}
		gitArgs = append(gitArgs, cfg.key, cfg.val)

		cmd := exec.Command("git", gitArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git config %s: %w", cfg.key, err)
		}
	}

	scope := "locally (in the current repository)"
	if global {
		scope = "globally"
		fmt.Println("WARNING: You have installed the extension globally.")
		fmt.Println("         All users cloning repositories that track files with this extension")
		fmt.Println("         must also have lfs-zstd-filter installed on their PATH.")
	}
	fmt.Printf("Successfully registered lfs-zstd-filter %s.\n", scope)
	return nil
}
