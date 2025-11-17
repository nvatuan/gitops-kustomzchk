package kustomize

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var logger = log.WithField("package", "kustomize")

var (
	// ErrOverlayNotFound indicates that the requested overlay/environment doesn't exist
	ErrOverlayNotFound = errors.New("overlay not found")
)

const (
	KUSTOMIZE_BASE_DIR         = "base"
	KUSTOMIZE_OVERLAY_DIR_NAME = "environments"
)

var (
	KUSTOMIZE_FILE_NAMES = []string{"kustomization.yaml", "kustomization.yml"}
)

// Expected structure for Kustomize building:
// - <manifestRoot>/
// |-- <service>/
// |   |-- <KUSTOMIZE_BASE_DIR>/
// |   |-- <KUSTOMIZE_OVERLAY_DIR_NAME>/
// |   |   |-- <overlayName>/
// |   |   |   |-- <kustomization.yaml / kustomization.yml>
// |   |   |   |-- <other files>
// |   |   |-- <overlayName_2>/
// |   |   |   |-- <kustomization.yaml / kustomization.yml>
// |   |   |   |-- <other files>

// KustomizeBuilder defines the interface for building Kubernetes manifests
type KustomizeBuilder interface {
	// Build runs kustomize build on the specified path
	// Path here is a full path to service (manifestRoot + service), kustomize will be built at path+overlay
	Build(ctx context.Context, path string, overlayName string) ([]byte, error)
	BuildToText(ctx context.Context, path string, overlayName string) (string, error)
}

// Builder handles kustomize builds
type Builder struct {
	FailOnOverlayNotFound bool // If true, fail when overlay doesn't exist; if false, skip gracefully
}

// Ensure Builder implements KustomizeBuilder
var _ KustomizeBuilder = (*Builder)(nil)

// NewBuilder creates a new kustomize builder with default settings (fail on overlay not found = false)
func NewBuilder() *Builder {
	return &Builder{
		FailOnOverlayNotFound: false,
	}
}

// NewBuilderWithOptions creates a new kustomize builder with custom options
func NewBuilderWithOptions(failOnOverlayNotFound bool) *Builder {
	return &Builder{
		FailOnOverlayNotFound: failOnOverlayNotFound,
	}
}

func (b *Builder) Build(ctx context.Context, path string, overlayName string) ([]byte, error) {
	buildPath, err := b.getBuildPath(path, overlayName)
	if err != nil {
		// If it's the special "overlay not found" error, propagate it up
		if errors.Is(err, ErrOverlayNotFound) {
			return nil, ErrOverlayNotFound
		}
		return nil, err
	}
	return b.buildAtPath(ctx, buildPath)
}

func (b *Builder) BuildToText(ctx context.Context, path string, overlayName string) (string, error) {
	bytes, err := b.Build(ctx, path, overlayName)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Build runs kustomize build on the specified path
// path here is fullpath to a service (manifestRoot + service)
func (b *Builder) buildAtPath(ctx context.Context, path string) ([]byte, error) {
	logger.WithField("path", path).Info("Building at path...")
	cmd := exec.CommandContext(ctx, "kustomize", "build", path)

	// Use Output() instead of CombinedOutput() to avoid stderr warnings in the output
	output, err := cmd.Output()
	if err != nil {
		// On error, get stderr for debugging
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("kustomize build failed: %w\nStderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("kustomize build failed: %w", err)
	}

	return output, nil
}

// GetServiceEnvironmentPath returns the path to build for a service/environment
// path here is fullpath to a service (manifestRoot + service)
func (b *Builder) getBuildPath(path string, overlayName string) (string, error) {
	if err := b.validateBuildPath(path, overlayName); err != nil {
		return "", err
	}
	return filepath.Join(path, KUSTOMIZE_OVERLAY_DIR_NAME, overlayName), nil
}

// ValidateServiceEnvironment checks if a service/environment combination exists
// path here is fullpath to a service (manifestRoot + service)
func (b *Builder) validateBuildPath(path, overlayName string) error {
	logger.WithField("path", path).WithField("overlayName", overlayName).Info("Validating build path...")

	// Check if service exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' not found", path)
	}

	// Check if base exists
	basePath := filepath.Join(path, KUSTOMIZE_BASE_DIR)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Errorf("base directory not found for path '%s'", path)
	}

	if !b.isKustomizeFileInPath(basePath) {
		return fmt.Errorf("no kustomization file found in base directory for path '%s'", path)
	}

	// Check if environment exists
	envPath := filepath.Join(path, KUSTOMIZE_OVERLAY_DIR_NAME, overlayName)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// Handle missing overlay based on configuration
		if b.FailOnOverlayNotFound {
			return fmt.Errorf("environment '%s' not found for path '%s'", overlayName, path)
		}
		// If we're not failing on missing overlays, return the special error
		logger.WithField("path", path).WithField("overlayName", overlayName).
			Warn("Environment overlay not found, will skip this environment")
		return ErrOverlayNotFound
	}

	// If overlay exists, it must be able to build
	if !b.isKustomizeFileInPath(envPath) {
		return fmt.Errorf("no kustomization file found in environment '%s' for path '%s'", overlayName, path)
	}
	return nil
}

func (b *Builder) isKustomizeFileInPath(kustomizeBuildPath string) bool {
	found := false
	for _, kustomizeFileName := range KUSTOMIZE_FILE_NAMES {
		kustomizeFilePath := filepath.Join(kustomizeBuildPath, kustomizeFileName)
		if _, err := os.Stat(kustomizeFilePath); err == nil {
			found = true
			break
		}
	}
	return found
}
