package zbxclient

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type sessionData struct {
	Token string `json:"token"`
}

func sessionFilePath(profileName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "zx", "sessions", profileName+".json")
}

func LoadSessionToken(profileName string) string {
	path := sessionFilePath(profileName)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s sessionData
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	return s.Token
}

func SaveSessionToken(profileName, token string) error {
	path := sessionFilePath(profileName)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(sessionData{Token: token})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ClearSessionToken(profileName string) error {
	path := sessionFilePath(profileName)
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
