package config

import (
	"fmt"
	"os"
)

func (cfg Config) GetFullScreenDiffPagerEnv() []string {
	diff := cfg.Pager.Diff
	if diff == "" {
		diff = "less"
	}
	if diff == "delta" {
		diff = "delta --paging always"
	}

	var env = os.Environ()
	env = append(
		env,
		"LESS=CRX",
		fmt.Sprintf(
			"GH_PAGER=%s",
			diff,
		),
	)

	return env
}

func (cfg PrsSectionConfig) ToSectionConfig() SectionConfig {
	return SectionConfig{
		Title:   cfg.Title,
		Filters: cfg.Filters,
		Limit:   cfg.Limit,
	}
}

func (cfg IssuesSectionConfig) ToSectionConfig() SectionConfig {
	return SectionConfig{
		Title:   cfg.Title,
		Filters: cfg.Filters,
		Limit:   cfg.Limit,
	}
}

// last defined value wins
func MergeColumnConfigs(configs ...ColumnConfig) ColumnConfig {
	colCfg := ColumnConfig{}

	for _, cfg := range configs {
		if cfg.Width != nil {
			colCfg.Width = cfg.Width
		}
		if cfg.Hidden != nil {
			colCfg.Hidden = cfg.Hidden
		}
		if cfg.Title != nil {
			colCfg.Title = cfg.Title
		}
	}

	return colCfg
}
