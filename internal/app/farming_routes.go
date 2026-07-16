package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func farmingRouteDirectory(cfg *config.Config, character, difficulty string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("farming route directory requires config")
	}
	character, err := validateOfflineCharacter(character)
	if err != nil {
		return "", err
	}
	parsedDifficulty, err := parseOfflineDifficulty(difficulty)
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg.ResolvePath(cfg.Routes.FarmingRoot), strings.ToLower(character), string(parsedDifficulty)), nil
}

func configuredFarmingRouteDirectory(cfg *config.Config) (string, error) {
	return farmingRouteDirectory(cfg, cfg.Session.Character, cfg.Session.Difficulty)
}
