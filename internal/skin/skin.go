package skin

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type SkinInfo struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"display_name"`
	Description         string `json:"description"`
	Type                string `json:"type"` // "builtin" | "custom"
	SpritesheetPath     string `json:"spritesheet_path,omitempty"`
	PetJSONPath         string `json:"pet_json_path,omitempty"`
	SpriteVersionNumber int    `json:"sprite_version_number"`
}

type PetManifest struct {
	ID                  string                 `json:"id"`
	DisplayName         string                 `json:"displayName"`
	Description         string                 `json:"description"`
	SpriteVersionNumber int                    `json:"spriteVersionNumber"`
	SpritesheetPath     string                 `json:"spritesheetPath"`
	Animations          map[string]interface{} `json:"animations,omitempty"`
}

var BuiltinSkins = []SkinInfo{
	{
		ID:                  "cat",
		DisplayName:         "橘宝喵",
		Description:         "软萌可爱的橘猫伴侣，活泼好动，拥有眨眼、跳跃、倾听等完整动作帧",
		Type:                "builtin",
		SpriteVersionNumber: 1,
	},
	{
		ID:                  "fox",
		DisplayName:         "灵狐小九",
		Description:         "仙气灵动的治愈系赤狐，摇着毛茸茸大尾巴，聪明乖巧",
		Type:                "builtin",
		SpriteVersionNumber: 1,
	},
	{
		ID:                  "spirit",
		DisplayName:         "星灵露露",
		Description:         "软萌发光的奇幻小水滴星灵，律动跳跃，温暖治愈",
		Type:                "builtin",
		SpriteVersionNumber: 1,
	},
}

func DefaultSkinsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "skins"
	}
	dir := filepath.Join(home, ".avatar", "skins")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

type Manager struct {
	skinsDir string
}

func NewManager(customSkinsDir string) *Manager {
	dir := customSkinsDir
	if dir == "" {
		dir = DefaultSkinsDir()
	}
	_ = os.MkdirAll(dir, 0755)
	return &Manager{skinsDir: dir}
}

func (m *Manager) ListSkins() []SkinInfo {
	list := make([]SkinInfo, 0, len(BuiltinSkins)+4)
	list = append(list, BuiltinSkins...)

	entries, err := os.ReadDir(m.skinsDir)
	if err != nil {
		return list
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skinDir := filepath.Join(m.skinsDir, entry.Name())
		info, err := readSkinDir(skinDir)
		if err == nil {
			list = append(list, info)
		}
	}

	return list
}

func readSkinDir(dir string) (SkinInfo, error) {
	manifestPath := filepath.Join(dir, "pet.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return SkinInfo{}, fmt.Errorf("missing pet.json: %w", err)
	}

	var m PetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return SkinInfo{}, fmt.Errorf("invalid pet.json: %w", err)
	}

	id := strings.TrimSpace(m.ID)
	if !validIDPattern.MatchString(id) {
		return SkinInfo{}, fmt.Errorf("invalid skin id in pet.json")
	}

	sheetName := strings.TrimSpace(m.SpritesheetPath)
	if sheetName == "" {
		sheetName = "spritesheet.webp"
	}
	sheetPath := filepath.Join(dir, sheetName)

	name := m.DisplayName
	if name == "" {
		name = id
	}

	version := m.SpriteVersionNumber
	if version <= 0 {
		version = 1
	}

	return SkinInfo{
		ID:                  id,
		DisplayName:         name,
		Description:         m.Description,
		Type:                "custom",
		SpritesheetPath:     sheetPath,
		PetJSONPath:         manifestPath,
		SpriteVersionNumber: version,
	}, nil
}

func (m *Manager) ImportSkin(sourcePath string) (SkinInfo, error) {
	fi, err := os.Stat(sourcePath)
	if err != nil {
		return SkinInfo{}, fmt.Errorf("skin source path not found: %w", err)
	}

	var sourceDir string
	var cleanupTmp string

	if fi.IsDir() {
		sourceDir = sourcePath
	} else if strings.EqualFold(filepath.Base(sourcePath), "pet.json") {
		sourceDir = filepath.Dir(sourcePath)
	} else if strings.EqualFold(filepath.Ext(sourcePath), ".zip") {
		tmpDir, err := os.MkdirTemp("", "avatar-skin-*")
		if err != nil {
			return SkinInfo{}, fmt.Errorf("failed to create temp dir: %w", err)
		}
		cleanupTmp = tmpDir
		defer func() {
			if cleanupTmp != "" {
				_ = os.RemoveAll(cleanupTmp)
			}
		}()

		if err := unzip(sourcePath, tmpDir); err != nil {
			return SkinInfo{}, fmt.Errorf("failed to unzip skin: %w", err)
		}

		located, err := locatePetRoot(tmpDir)
		if err != nil {
			return SkinInfo{}, err
		}
		sourceDir = located
	} else {
		return SkinInfo{}, fmt.Errorf("unsupported skin source, expect directory, pet.json or .zip")
	}

	info, err := readSkinDir(sourceDir)
	if err != nil {
		return SkinInfo{}, fmt.Errorf("invalid skin package: %w", err)
	}

	destDir := filepath.Join(m.skinsDir, info.ID)
	_ = os.RemoveAll(destDir)
	if err := copyDir(sourceDir, destDir); err != nil {
		return SkinInfo{}, fmt.Errorf("failed to copy skin to target dir: %w", err)
	}

	return readSkinDir(destDir)
}

func locatePetRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "pet.json")); err == nil {
		return dir, nil
	}

	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				sub := filepath.Join(dir, e.Name())
				if _, err := os.Stat(filepath.Join(sub, "pet.json")); err == nil {
					return sub, nil
				}
			}
		}
	}
	return "", fmt.Errorf("pet.json not found in package")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func unzip(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.Contains(f.Name, "..") {
			continue // prevent zip slip
		}
		target := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		outFile, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
