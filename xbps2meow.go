package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

const (
	xbpsDBDir = "/var/db/xbps"
	meowDBDir = "/var/db/meow"
)

type FileInfo struct {
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256,omitempty"`
}

type Pkg struct {
	Pkgname      string              `json:"pkgname"`
	Version      string              `json:"version"`
	Architecture string              `json:"architecture"`
	ShortDesc    string              `json:"short_desc"`
	RunDepends   []string            `json:"run_depends"`
	Provides     []string            `json:"provides"`
	Conflicts    []string            `json:"conflicts"`
	Files        map[string]FileInfo `json:"files"`
	EtcFiles     []string            `json:"etc_files"`
	InstallDate  string              `json:"install_date"`
	Auto         bool                `json:"auto"`
	Status       string              `json:"status"`
}

type Installed struct {
	Name    string
	Version string
	Date    string
	Auto    bool
}

func dictStr(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func dictStrs(m map[string]interface{}, k string) []string {
	arr, ok := m[k].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func parseFilesPlist(path string) map[string]FileInfo {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := map[string]FileInfo{}
	// files.plist: {"files": [...], "conf_files": [...], "links": [...]}.
	// Путь в ключе "file" (не "path"), нормализуем к "/usr/bin/foo" с ведущим слэшем,
	// чтобы совпадало с meow-форматом.
	for _, key := range []string{"files", "conf_files"} {
		arr, ok := raw[key].([]interface{})
		if !ok {
			continue
		}
		for _, e := range arr {
			m, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			p := dictStr(m, "file")
			if p == "" {
				continue
			}
			p = strings.TrimPrefix(p, "./")
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			out[p] = FileInfo{
				Mode:   dictStr(m, "mode"),
				SHA256: dictStr(m, "sha256"),
			}
		}
	}
	return out
}

func readInstalled() []Installed {
	f, err := os.Open(filepath.Join(meowDBDir, "installed.list"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Installed
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Split(sc.Text(), "|")
		if len(p) == 4 {
			out = append(out, Installed{p[0], p[1], p[2], p[3] == "1"})
		}
	}
	return out
}

func writeInstalled(es []Installed) error {
	var b strings.Builder
	for _, e := range es {
		a := "0"
		if e.Auto {
			a = "1"
		}
		b.WriteString(e.Name)
		b.WriteByte('|')
		b.WriteString(e.Version)
		b.WriteByte('|')
		b.WriteString(e.Date)
		b.WriteByte('|')
		b.WriteString(a)
		b.WriteByte('\n')
	}
	tmp := filepath.Join(meowDBDir, "installed.list.tmp")
	if err := os.WriteFile(tmp, []byte(b.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(meowDBDir, "installed.list"))
}

func writePkg(p *Pkg) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(meowDBDir, p.Pkgname+".json"), data, 0644)
}

func splitPkgver(pkgver, pkgname string) string {
	if strings.HasPrefix(pkgver, pkgname+"-") {
		return pkgver[len(pkgname)+1:]
	}
	return pkgver
}

// extractPackages вытаскивает список пакетов из распарсенного plist
// вне зависимости от формы (map-of-dicts, dict-with-array, list-of-dicts).
func extractPackages(v interface{}) []map[string]interface{} {
	var out []map[string]interface{}

	switch t := v.(type) {
	case map[string]interface{}:
		// Вариант 1: {pkgname: {props...}, ...}
		for _, val := range t {
			if m, ok := val.(map[string]interface{}); ok {
				if _, has := m["pkgname"]; has {
					out = append(out, m)
				}
			}
		}
		// Вариант 2: {packages: [{...}, ...]}
		if len(out) == 0 {
			for _, val := range t {
				if arr, ok := val.([]interface{}); ok {
					for _, e := range arr {
						if m, ok := e.(map[string]interface{}); ok {
							if _, has := m["pkgname"]; has {
								out = append(out, m)
							}
						}
					}
				}
			}
		}
	case []interface{}:
		// Вариант 3: [{...}, ...]
		for _, e := range t {
			if m, ok := e.(map[string]interface{}); ok {
				if _, has := m["pkgname"]; has {
					out = append(out, m)
				}
			}
		}
	}
	return out
}

func main() {
	pkgdbPath := filepath.Join(xbpsDBDir, "pkgdb-0.38.plist")
	data, err := os.ReadFile(pkgdbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read %s: %v\n", pkgdbPath, err)
		os.Exit(1)
	}

	var raw interface{}
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "plist parse: %v\n", err)
		os.Exit(1)
	}

	pkgsRaw := extractPackages(raw)
	fmt.Printf("pkgdb: %d packages\n", len(pkgsRaw))
	if len(pkgsRaw) == 0 {
		fmt.Fprintln(os.Stderr, "no packages extracted; inspect pkgdb-0.38.plist format")
		os.Exit(1)
	}

	if err := os.MkdirAll(meowDBDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	installed := readInstalled()
	seen := map[string]bool{}
	for _, e := range installed {
		seen[e.Name] = true
	}

	added, skipped, failed := 0, 0, 0
	for _, m := range pkgsRaw {
		name := dictStr(m, "pkgname")
		if name == "" {
			continue
		}
		if seen[name] {
			skipped++
			continue
		}

		pkgver := dictStr(m, "pkgver")
		version := splitPkgver(pkgver, name)

		filesPath := filepath.Join(xbpsDBDir, "."+name+"-files.plist")
		files := parseFilesPlist(filesPath)
		var etcFiles []string
		for f := range files {
			if strings.HasPrefix(f, "/etc/") {
				etcFiles = append(etcFiles, f)
			}
		}

		auto := false
		if v, ok := m["automatic-install"].(bool); ok {
			auto = v
		}

		pkg := &Pkg{
			Pkgname:      name,
			Version:      version,
			Architecture: dictStr(m, "architecture"),
			ShortDesc:    dictStr(m, "short_desc"),
			RunDepends:   dictStrs(m, "run_depends"),
			Provides:     dictStrs(m, "provides"),
			Conflicts:    dictStrs(m, "conflicts"),
			Files:        files,
			EtcFiles:     etcFiles,
			InstallDate:  time.Now().UTC().Format(time.RFC3339),
			Auto:         auto,
			Status:       "installed",
		}

		if err := writePkg(pkg); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", name, err)
			failed++
			continue
		}

		installed = append(installed, Installed{
			Name:    name,
			Version: version,
			Date:    pkg.InstallDate,
			Auto:    auto,
		})
		added++
	}

	if err := writeInstalled(installed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("converted: %d, skipped: %d, failed: %d\n", added, skipped, failed)
}
