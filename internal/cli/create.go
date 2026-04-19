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

func newCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Interactive wizard to create a new connection",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			conns, err := listConnections()
			if err != nil {
				return err
			}

			result := tui.Run(conns, sharedDB, tui.TabCreate, runtimeCfg)
			if result == nil || result.Action == tui.ActionNone {
				return nil
			}

			switch result.Action {
			case tui.ActionConnect:
				return connect(result.Connection, nil)
			case tui.ActionCreated:
				return savePasswordAuth(sharedDB, result.WizardResult)
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
		Name:      wr.Name,
		User:      wr.User,
		Host:      wr.Host,
		Port:      port,
		Directory: wr.Directory,
		ProxyJump: wr.ProxyJump,
		AuthType:  model.AuthPassword,
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

// encryptPassword handles the full passphrase → encryption flow for CLI usage.
// On first call: prompts to create a master passphrase, generates an Argon2id salt,
// and stores an encrypted verification token.
// On subsequent calls: prompts for the existing passphrase and verifies it.
// Returns the encrypted SSH password and its AES-GCM nonce.
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
		// Store verification token.
		key := crypto.DeriveKey(passphrase, salt)
		chk, chkNonce, err := crypto.Encrypt(key, []byte("dssh-verify"))
		if err != nil {
			return nil, nil, err
		}
		if err := db.SetSetting(d, "passphrase_check", append(chkNonce, chk...)); err != nil {
			return nil, nil, err
		}
	} else {
		fmt.Print("Enter master passphrase: ")
		passphrase, err = readPassword()
		if err != nil {
			return nil, nil, err
		}
		// Verify passphrase against stored token.
		if err := verifyPassphrase(d, passphrase, salt); err != nil {
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
	if len(conn.EncryptedPass) == 0 || len(conn.PassNonce) == 0 {
		return "", fmt.Errorf("no encrypted password stored for %q", conn.Name)
	}

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

	// Verify passphrase against stored token.
	if err := verifyPassphrase(d, passphrase, salt); err != nil {
		return "", err
	}

	key := crypto.DeriveKey(passphrase, salt)
	plaintext, err := crypto.Decrypt(key, conn.EncryptedPass, conn.PassNonce)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong passphrase?): %w", err)
	}

	return string(plaintext), nil
}

// verifyPassphrase checks the entered passphrase against the stored verification token.
func verifyPassphrase(d *sql.DB, passphrase string, salt []byte) error {
	chkData, err := db.GetSetting(d, "passphrase_check")
	if err != nil {
		return err
	}
	if chkData == nil {
		// No verification token stored (legacy data) — skip verification.
		return nil
	}
	// chkData = nonce + ciphertext; nonce is 12 bytes for AES-GCM.
	if len(chkData) <= 12 {
		return fmt.Errorf("corrupted passphrase verification data")
	}
	chkNonce := chkData[:12]
	chkCiphertext := chkData[12:]
	key := crypto.DeriveKey(passphrase, salt)
	plain, err := crypto.Decrypt(key, chkCiphertext, chkNonce)
	if err != nil {
		return fmt.Errorf("wrong master passphrase")
	}
	if string(plain) != "dssh-verify" {
		return fmt.Errorf("wrong master passphrase")
	}
	return nil
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
