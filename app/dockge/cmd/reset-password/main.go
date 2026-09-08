package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"dockge/pkg/config"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	var confPath = flag.String("conf", "config/dockge/local.yml", "config path")
	flag.Parse()

	conf, err := config.New(*confPath)
	if err != nil {
		panic(err)
	}

	dsn := conf.GetString("data.db.user.dsn")
	if dsn == "" {
		dsn = "storage/dockge.db"
	}
	db, err := bbolt.Open(dsn, 0o600, nil)
	if err != nil {
		panic(fmt.Errorf("open db: %w", err))
	}
	defer db.Close()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter username to reset: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		fmt.Println("Error: username is required")
		os.Exit(1)
	}

	newPassword := ""
	for {
		fmt.Print("Enter new password: ")
		pw, _ := reader.ReadString('\n')
		pw = strings.TrimSpace(pw)
		fmt.Print("Confirm new password: ")
		pw2, _ := reader.ReadString('\n')
		pw2 = strings.TrimSpace(pw2)
		if pw == pw2 && len(pw) >= 6 {
			newPassword = pw
			break
		}
		fmt.Println("Passwords do not match or too short (min 6 chars)")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	found := false
	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return fmt.Errorf("users bucket not found")
		}
		cursor := b.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var user struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.Unmarshal(v, &user); err != nil {
				continue
			}
			if user.Username == username {
				user.Password = string(hashed)
				data, _ := json.Marshal(user)
				if err := b.Put(k, data); err != nil {
					return err
				}
				found = true
				break
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	if !found {
		fmt.Printf("User '%s' not found\n", username)
		os.Exit(1)
	}

	fmt.Printf("Password for '%s' has been reset successfully\n", username)
}
