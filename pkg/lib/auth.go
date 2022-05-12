package lib

import "fmt"

// Credential returns the credential to Allegro Zipper.
func Credential(appID string, appSecret string) string {
	return fmt.Sprintf("app-key-secret:%s|%s", appID, appSecret)
}