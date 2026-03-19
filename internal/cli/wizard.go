package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/madLinux7/dssh/internal/crypto"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newWizardCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "wizard",
		Aliases: []string{"new"},
		Short:   "Interactive wizard to create a new connection",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()

			conns, err := db.List(d)
			if err != nil {
				return err
			}

			result := tui.Run(conns, d, tui.TabNew)
			if result == nil || result.Action == tui.ActionNone {
				return nil
			}

			switch result.Action {
			case tui.ActionConnect:
				return connect(d, result.Connection, nil)
			case tui.ActionCreated:
				return savePasswordAuth(d, result.WizardResult)
			}
			return nil
		},
	}
}

// savePasswordAuth encrypts the password and saves the connection.
// Only called for password-auth connections — validation was already
// performed by the TUI's handleSave.
func savePasswordAuth(d *sql.DB, wr *tui.WizardResult) error {
	port, _ := strconv.Atoi(wr.Port) // already validated by TUI

	conn := &model.Connection{
		Name:     wr.Name,
		User:     wr.User,
		Host:     wr.Host,
		Port:     port,
		AuthType: model.AuthPassword,
	}

	if wr.Password != "" {
		encPass, nonce, err := encryptPassword(d, wr.Password)
		if err != nil {
			return err
		}
		conn.EncryptedPass = encPass
		conn.PassNonce = nonce
	}

	if err := db.Insert(d, conn); err != nil {
		return err
	}

	success("Added connection %q (%s@%s:%d)", conn.Name, conn.User, conn.Host, conn.Port)
	return nil
}

// encryptPassword handles master passphrase creation/prompting and encrypts the SSH password.
func encryptPassword(d *sql.DB, password string) ([]byte, []byte, error) {
	salt, err := db.GetSetting(d, "argon2_salt")
	if err != nil {
		return nil, nil, err
	}

	var passphrase string
	if salt == nil {
		// First time — create master passphrase.
		fmt.Println("Create a master passphrase to encrypt stored passwords (Don't forget it or else you will need to reset the app!).")
		passphrase, err = promptPassphraseTwice()
		if err != nil {
			return nil, nil, err
		}
		salt, err = crypto.GenerateSalt()
		if err != nil {
			return nil, nil, err
		}
		if err := db.SetSetting(d, "argon2_salt", salt); err != nil {
			return nil, nil, err
		}
	} else {
		fmt.Print("Enter master passphrase: ")
		passphrase, err = readPassword()
		if err != nil {
			return nil, nil, err
		}
	}

	key := crypto.DeriveKey(passphrase, salt)
	ciphertext, nonce, err := crypto.Encrypt(key, []byte(password))
	if err != nil {
		return nil, nil, err
	}

	return ciphertext, nonce, nil
}

// decryptPassword prompts for the master passphrase and decrypts a stored password.
func decryptPassword(d *sql.DB, conn *model.Connection) (string, error) {
	salt, err := db.GetSetting(d, "argon2_salt")
	if err != nil {
		return "", err
	}
	if salt == nil {
		return "", fmt.Errorf("no master passphrase configured — cannot decrypt")
	}

	fmt.Print("Enter master passphrase: ")
	passphrase, err := readPassword()
	if err != nil {
		return "", err
	}

	key := crypto.DeriveKey(passphrase, salt)
	plaintext, err := crypto.Decrypt(key, conn.EncryptedPass, conn.PassNonce)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong passphrase?): %w", err)
	}

	return string(plaintext), nil
}

func promptPassphraseTwice() (string, error) {
	fmt.Print("Enter passphrase: ")
	p1, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Print("Confirm passphrase: ")
	p2, err := readPassword()
	if err != nil {
		return "", err
	}
	if p1 != p2 {
		return "", fmt.Errorf("passphrases do not match")
	}
	if p1 == "" {
		return "", fmt.Errorf("passphrase cannot be empty")
	}
	return p1, nil
}

func readPassword() (string, error) {
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	return string(b), nil
}
