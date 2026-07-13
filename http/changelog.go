package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

type changelogEntry struct {
	Date    string   `json:"date"`
	Version string   `json:"version"`
	Changes []string `json:"changes"`
}

func changelogFilePath() string {
	if path := os.Getenv("CHANGELOG_PATH"); path != "" {
		return path
	}
	return "changelog.json"
}

func loadChangelogEntries() ([]changelogEntry, error) {
	path := changelogFilePath()
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc struct {
		Entries []changelogEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc.Entries == nil {
		doc.Entries = []changelogEntry{}
	}
	return doc.Entries, nil
}

func handleChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET",
		})
		return
	}

	entries, err := loadChangelogEntries()
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"ok":    false,
				"error": "未找到变更记录文件：" + filepath.Base(changelogFilePath()),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"file":    changelogFilePath(),
		"entries": entries,
	})
}
