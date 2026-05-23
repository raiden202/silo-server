package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

func TestInstallerReplaceBinaryPreservesInstallationID(t *testing.T) {
	ctx := context.Background()

	oldDir := t.TempDir()
	oldPath := filepath.Join(oldDir, "plugin")
	if err := os.WriteFile(oldPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", oldPath, err)
	}

	store := newRecordingInstallationStore()
	installer := NewInstaller(store, InstallerOptions{BaseDir: t.TempDir()})

	manifest := testPluginManifest(t, "silo.metadb", "0.0.19")
	binaryData := []byte("#!/bin/sh\nexit 0\n")
	checksum := sha256.Sum256(binaryData)

	result, err := installer.replaceBinary(ctx, &Installation{
		ID:          15,
		PluginID:    "silo.metadb",
		Version:     "0.0.18",
		InstallPath: oldPath,
		Enabled:     true,
	}, binaryData, hex.EncodeToString(checksum[:]), manifest)
	if err != nil {
		t.Fatalf("replaceBinary() returned error: %v", err)
	}

	if len(store.createInputs) != 0 {
		t.Fatalf("Create() called %d times, want 0", len(store.createInputs))
	}
	if len(store.saveArchiveIDs) != 1 || store.saveArchiveIDs[0] != 15 {
		t.Fatalf("SaveArchive() ids = %#v, want [15]", store.saveArchiveIDs)
	}
	if len(store.updateIDs) != 1 || store.updateIDs[0] != 15 {
		t.Fatalf("Update() ids = %#v, want [15]", store.updateIDs)
	}
	if result.Installation.ID != 15 {
		t.Fatalf("result installation id = %d, want 15", result.Installation.ID)
	}
	if result.Installation.Version != "0.0.19" {
		t.Fatalf("result installation version = %q, want 0.0.19", result.Installation.Version)
	}
	if result.Installation.InstallPath == oldPath {
		t.Fatal("expected replaceBinary() to move the installation to a new path")
	}
	if result.Installation.InstallPath != result.BinaryPath {
		t.Fatalf("result install path = %q, want binary path %q", result.Installation.InstallPath, result.BinaryPath)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("expected old installation dir to be removed, stat error = %v", err)
	}
	if _, err := os.Stat(result.BinaryPath); err != nil {
		t.Fatalf("expected replaced plugin binary to exist: %v", err)
	}
	if _, err := os.Stat(result.ManifestPath); err != nil {
		t.Fatalf("expected replaced manifest to exist: %v", err)
	}

	update := store.updateInputs[0]
	if update.Version == nil || *update.Version != "0.0.19" {
		t.Fatalf("update version = %v, want 0.0.19", update.Version)
	}
	if update.InstallPath == nil || *update.InstallPath != result.BinaryPath {
		t.Fatalf("update install_path = %v, want %q", update.InstallPath, result.BinaryPath)
	}
	if update.Enabled == nil || !*update.Enabled {
		t.Fatalf("update enabled = %v, want true", update.Enabled)
	}
	if len(update.Capabilities) != 1 || update.Capabilities[0].ID != "metadb" {
		t.Fatalf("update capabilities = %#v, want metadata_provider.v1/metadb", update.Capabilities)
	}
}

func testPluginManifest(t *testing.T, pluginID, version string) *pluginv1.PluginManifest {
	t.Helper()

	manifest := &pluginv1.PluginManifest{
		PluginId:       pluginID,
		Version:        version,
		Checksum:       "sha256-placeholder",
		SiloApiVersion: DefaultSiloAPIVersion,
		SupportedPlatforms: []*pluginv1.SupportedPlatform{
			{Os: "darwin", Arch: "arm64"},
		},
		Capabilities: []*pluginv1.CapabilityDescriptor{
			{
				Type:        "metadata_provider.v1",
				Id:          "metadb",
				DisplayName: "MetaDB",
			},
		},
	}

	if _, err := protojson.Marshal(manifest); err != nil {
		t.Fatalf("protojson.Marshal() returned error: %v", err)
	}

	return manifest
}

type recordingInstallationStore struct {
	createInputs        []CreateInstallationInput
	updateIDs           []int
	updateInputs        []UpdateInstallationInput
	deleteIDs           []int
	saveArchiveIDs      []int
	saveArchiveErr      error
	saveArchiveManifest []byte
	saveArchiveChecksum string
	saveArchiveBytes    []byte
	updateErr           error
}

func newRecordingInstallationStore() *recordingInstallationStore {
	return &recordingInstallationStore{}
}

func (s *recordingInstallationStore) Create(_ context.Context, input CreateInstallationInput) (*Installation, error) {
	s.createInputs = append(s.createInputs, input)
	return &Installation{
		ID:          1,
		PluginID:    input.PluginID,
		Version:     input.Version,
		InstallPath: input.InstallPath,
		Enabled:     input.Enabled,
	}, nil
}

func (s *recordingInstallationStore) SaveArchive(
	_ context.Context,
	installationID int,
	manifestJSON []byte,
	checksum string,
	archiveBytes []byte,
) error {
	s.saveArchiveIDs = append(s.saveArchiveIDs, installationID)
	s.saveArchiveManifest = append([]byte(nil), manifestJSON...)
	s.saveArchiveChecksum = checksum
	s.saveArchiveBytes = append([]byte(nil), archiveBytes...)
	return s.saveArchiveErr
}

func (s *recordingInstallationStore) Update(_ context.Context, id int, input UpdateInstallationInput) error {
	s.updateIDs = append(s.updateIDs, id)
	s.updateInputs = append(s.updateInputs, input)
	return s.updateErr
}

func (s *recordingInstallationStore) Delete(_ context.Context, id int) error {
	s.deleteIDs = append(s.deleteIDs, id)
	return nil
}
