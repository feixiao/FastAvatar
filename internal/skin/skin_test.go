package skin

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinSkins(t *testing.T) {
	mgr := NewManager(t.TempDir())
	skins := mgr.ListSkins()
	if len(skins) < 3 {
		t.Fatalf("expected at least 3 builtin skins, got %d", len(skins))
	}
	if skins[0].ID != "cat" || skins[1].ID != "fox" || skins[2].ID != "spirit" {
		t.Errorf("builtin skins order or IDs mismatch: %+v", skins)
	}
}

func TestImportSkinFromDir(t *testing.T) {
	tmpDir := t.TempDir()
	sourceSkin := filepath.Join(tmpDir, "my-pet")
	if err := os.MkdirAll(sourceSkin, 0755); err != nil {
		t.Fatal(err)
	}

	petJSON := `{
		"id": "my-custom-pet",
		"displayName": "小恐龙",
		"description": "一只绿色的小恐龙",
		"spriteVersionNumber": 1,
		"spritesheetPath": "spritesheet.png"
	}`
	if err := os.WriteFile(filepath.Join(sourceSkin, "pet.json"), []byte(petJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceSkin, "spritesheet.png"), []byte("fake-png-data"), 0644); err != nil {
		t.Fatal(err)
	}

	targetSkinsDir := filepath.Join(tmpDir, "app-skins")
	mgr := NewManager(targetSkinsDir)

	imported, err := mgr.ImportSkin(sourceSkin)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if imported.ID != "my-custom-pet" {
		t.Errorf("expected ID 'my-custom-pet', got '%s'", imported.ID)
	}
	if imported.DisplayName != "小恐龙" {
		t.Errorf("expected DisplayName '小恐龙', got '%s'", imported.DisplayName)
	}

	list := mgr.ListSkins()
	var found bool
	for _, s := range list {
		if s.ID == "my-custom-pet" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("imported skin not found in ListSkins")
	}
}

func TestImportSkinFromZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "skin.zip")

	// Create zip
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)

	w1, _ := zw.Create("pet.json")
	_, _ = w1.Write([]byte(`{"id":"zip-pet","displayName":"压缩包萌宠"}`))
	w2, _ := zw.Create("spritesheet.webp")
	_, _ = w2.Write([]byte("fake-webp"))
	_ = zw.Close()
	_ = zf.Close()

	mgr := NewManager(filepath.Join(tmpDir, "skins"))
	imported, err := mgr.ImportSkin(zipPath)
	if err != nil {
		t.Fatalf("import from zip failed: %v", err)
	}
	if imported.ID != "zip-pet" {
		t.Errorf("expected ID 'zip-pet', got '%s'", imported.ID)
	}
}
