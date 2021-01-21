package engine

import (
	"os"
	"testing"

	"github.com/vazha/gocryptotrader/config"
)

func TestLoadConfigWithSettings(t *testing.T) {
	empty := ""
	somePath := "somePath"
	// Clean up after the tests
	defer os.RemoveAll(somePath)
	tests := []struct {
		name     string
		flags    []string
		settings *Settings
		want     *string
		wantErr  bool
	}{
		{
			name: "invalid file",
			settings: &Settings{
				ConfigFile: "nonExistent.json",
			},
			wantErr: true,
		},
		{
			name: "test file",
			settings: &Settings{
				ConfigFile:   config.TestFile,
				EnableDryRun: true,
			},
			want:    &empty,
			wantErr: false,
		},
		{
			name:  "data dir in settings overrides config data dir",
			flags: []string{"datadir"},
			settings: &Settings{
				ConfigFile:   config.TestFile,
				DataDir:      somePath,
				EnableDryRun: true,
			},
			want:    &somePath,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// prepare the 'flags'
			flagSet := make(map[string]bool)
			for _, v := range tt.flags {
				flagSet[v] = true
			}
			// Run the test
			got, err := loadConfigWithSettings(tt.settings, flagSet)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfigWithSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil || tt.want != nil {
				if (got == nil && tt.want != nil) || (got != nil && tt.want == nil) {
					t.Errorf("loadConfigWithSettings() = is nil %v, want nil %v", got == nil, tt.want == nil)
				} else if got.DataDirectory != *tt.want {
					t.Errorf("loadConfigWithSettings() = %v, want %v", got.DataDirectory, *tt.want)
				}
			}
		})
	}
}

func TestStartStopDoesNotCausePanic(t *testing.T) {
	t.Parallel()
	botOne, err := NewFromSettings(&Settings{
		ConfigFile:   config.TestFile,
		EnableDryRun: true,
	}, nil)
	if err != nil {
		t.Error(err)
	}

	if err = botOne.Start(); err != nil {
		t.Error(err)
	}

	botOne.Stop()
}

func TestStartStopTwoDoesNotCausePanic(t *testing.T) {
	t.Skip("Closing global currency.storage from two bots causes panic")
	t.Parallel()
	botOne, err := NewFromSettings(&Settings{
		ConfigFile:   config.TestFile,
		EnableDryRun: true,
	}, nil)
	if err != nil {
		t.Error(err)
	}
	botTwo, err := NewFromSettings(&Settings{
		ConfigFile:   config.TestFile,
		EnableDryRun: true,
	}, nil)
	if err != nil {
		t.Error(err)
	}
	if err = botOne.Start(); err != nil {
		t.Error(err)
	}
	if err = botTwo.Start(); err != nil {
		t.Error(err)
	}

	botOne.Stop()
	botTwo.Stop()
}
