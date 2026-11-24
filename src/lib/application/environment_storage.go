package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	secretcontract "dockpipe/src/lib/domain/secretenv"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/secretenv"
	"dockpipe/src/lib/model/managedenvironment"
)

const environmentStorageProfileSchema = "dockpipe.environment.storage-profile/v1"

type environmentStorageProfile struct {
	Schema   string `json:"schema"`
	Resolver string `json:"resolver"`
	Secrets  struct {
		Resolver   string            `json:"resolver"`
		Parameters map[string]string `json:"parameters"`
		Bindings   map[string]string `json:"bindings"`
	} `json:"secrets"`
}

func prepareEnvironmentStorage(storage *managedenvironment.Storage, environ []string) ([]string, error) {
	environ = withoutEnvironmentRuntimeVariables(environ)
	if storage == nil {
		return environ, nil
	}

	root, err := infrastructure.GlobalDockpipeDataDir()
	if err != nil {
		return nil, err
	}
	// All resolver discovery is rooted in the host-owned profile directory,
	// away from any repository that merely references this storage profile.
	profileDirectory := filepath.Join(root, "environment-storage")
	profile, err := readEnvironmentStorageProfile(profileDirectory, storage.Profile)
	if err != nil {
		return nil, err
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(profileDirectory, profile.Resolver)
	if err != nil {
		return nil, errors.New("environment storage resolver is unavailable")
	}
	resolver, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return nil, errors.New("cannot read the environment storage resolver profile")
	}
	hook, err := infrastructure.ResolveResolverHook(profilePath, resolver["DOCKPIPE_ENVIRONMENT_STORAGE_RESTORE"])
	if err != nil {
		return nil, err
	}

	cacheRoot, err := infrastructure.GlobalCacheRoot()
	if err != nil {
		return nil, err
	}
	cache := filepath.Join(cacheRoot, "environment-images")
	if err := infrastructure.PreparePrivateDirectory(cache); err != nil {
		return nil, err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	values, err := secretenv.Resolve(ctx, profileDirectory, profile.Secrets.Resolver, secretcontract.Request{
		Schema: secretcontract.Schema, Parameters: profile.Secrets.Parameters, Bindings: profile.Secrets.Bindings,
	})
	if err != nil {
		return nil, err
	}
	defer clear(values)
	base, err := restoreEnvironmentImage(ctx, hook, profileDirectory, cache, values)
	if err != nil {
		return nil, err
	}
	return append(environ, environmentImageBaseKey+"="+base.Path, environmentImageDigestKey+"="+base.SHA256), nil
}

func readEnvironmentStorageProfile(directory, name string) (environmentStorageProfile, error) {
	var profile environmentStorageProfile
	if !managedenvironment.ValidName(name) {
		return profile, errors.New("invalid environment storage profile name")
	}
	if err := infrastructure.ValidatePrivatePath(directory, true); err != nil {
		return profile, errors.New("environment storage requires a private host profile directory")
	}
	path := filepath.Join(directory, name+".json")
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return profile, errors.New("environment storage profile is missing or is not owner-only")
	}
	file, err := os.Open(path)
	if err != nil {
		return profile, errors.New("cannot open environment storage profile")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxEnvironmentDefinitionBytes+1))
	if err != nil || len(data) > maxEnvironmentDefinitionBytes {
		return profile, errors.New("cannot read the bounded environment storage profile")
	}
	defer clear(data)
	if err := decodeStorageJSON(data, &profile); err != nil || profile.Schema != environmentStorageProfileSchema {
		return profile, errors.New("invalid environment storage profile")
	}
	if !managedenvironment.ValidName(profile.Resolver) || !managedenvironment.ValidName(profile.Secrets.Resolver) {
		return profile, errors.New("environment storage requires named storage and secret resolvers")
	}
	if err := secretcontract.ValidateBindings(profile.Secrets.Bindings); err != nil {
		return profile, errors.New("environment storage requires valid secret bindings")
	}
	return profile, nil
}

func decodeStorageJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after JSON object")
	}
	return nil
}

const environmentImageBaseKey = "DOCKPIPE_ENVIRONMENT_IMAGE_BASE"
const environmentImageDigestKey = "DOCKPIPE_ENVIRONMENT_IMAGE_SHA256"

func withoutEnvironmentRuntimeVariables(environ []string) []string {
	filtered := make([]string, 0, len(environ)+4)
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case environmentImageBaseKey, environmentImageDigestKey, environmentInstanceKey, environmentStateWorkdirKey:
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
