package pathbuilder

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{
			name:     "single variable",
			template: "/path/$SERVICE/env",
			want:     []string{"SERVICE"},
		},
		{
			name:     "multiple variables",
			template: "/path/$SERVICE/clusters/$CLUSTER/$ENV",
			want:     []string{"SERVICE", "CLUSTER", "ENV"},
		},
		{
			name:     "no variables",
			template: "/path/to/kustomization",
			want:     nil,
		},
		{
			name:     "duplicate variables",
			template: "/path/$SERVICE/$SERVICE",
			want:     []string{"SERVICE"},
		},
		{
			name:     "variable with underscores",
			template: "/path/$MY_SERVICE/env",
			want:     []string{"MY_SERVICE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTemplate(tt.template)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseValues(t *testing.T) {
	tests := []struct {
		name      string
		valuesStr string
		want      map[string][]string
		wantErr   bool
	}{
		{
			name:      "single key single value",
			valuesStr: "SERVICE=my-app",
			want:      map[string][]string{"SERVICE": {"my-app"}},
			wantErr:   false,
		},
		{
			name:      "single key multiple values",
			valuesStr: "ENV=stg,prod",
			want:      map[string][]string{"ENV": {"stg", "prod"}},
			wantErr:   false,
		},
		{
			name:      "multiple keys",
			valuesStr: "SERVICE=my-app;CLUSTER=alpha,beta;ENV=stg,prod",
			want: map[string][]string{
				"SERVICE": {"my-app"},
				"CLUSTER": {"alpha", "beta"},
				"ENV":     {"stg", "prod"},
			},
			wantErr: false,
		},
		{
			name:      "empty string",
			valuesStr: "",
			want:      map[string][]string{},
			wantErr:   false,
		},
		{
			name:      "with spaces",
			valuesStr: "SERVICE = my-app ; ENV = stg , prod",
			want: map[string][]string{
				"SERVICE": {"my-app"},
				"ENV":     {"stg", "prod"},
			},
			wantErr: false,
		},
		{
			name:      "invalid format no equals",
			valuesStr: "SERVICE",
			wantErr:   true,
		},
		{
			name:      "empty key",
			valuesStr: "=value",
			wantErr:   true,
		},
		{
			name:      "empty value",
			valuesStr: "SERVICE=",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseValues(tt.valuesStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathBuilder_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pb      *PathBuilder
		wantErr bool
	}{
		{
			name: "valid - all vars have values",
			pb: &PathBuilder{
				Template: "/path/$SERVICE/$ENV",
				Variables: map[string][]string{
					"SERVICE": {"my-app"},
					"ENV":     {"stg", "prod"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing variable value",
			pb: &PathBuilder{
				Template: "/path/$SERVICE/$ENV",
				Variables: map[string][]string{
					"SERVICE": {"my-app"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty template",
			pb: &PathBuilder{
				Template:  "",
				Variables: map[string][]string{},
			},
			wantErr: true,
		},
		{
			name: "no variables in template",
			pb: &PathBuilder{
				Template:  "/path/to/dir",
				Variables: map[string][]string{},
			},
			wantErr: false,
		},
		{
			name: "empty values array",
			pb: &PathBuilder{
				Template: "/path/$SERVICE",
				Variables: map[string][]string{
					"SERVICE": {},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pb.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathBuilder_InterpolatePath(t *testing.T) {
	tests := []struct {
		name    string
		pb      *PathBuilder
		values  map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "single variable",
			pb: &PathBuilder{
				Template: "/path/$SERVICE/env",
			},
			values:  map[string]string{"SERVICE": "my-app"},
			want:    "/path/my-app/env",
			wantErr: false,
		},
		{
			name: "multiple variables",
			pb: &PathBuilder{
				Template: "/path/$SERVICE/clusters/$CLUSTER/$ENV",
			},
			values: map[string]string{
				"SERVICE": "my-app",
				"CLUSTER": "alpha",
				"ENV":     "stg",
			},
			want:    "/path/my-app/clusters/alpha/stg",
			wantErr: false,
		},
		{
			name: "unresolved variable",
			pb: &PathBuilder{
				Template: "/path/$SERVICE/$ENV",
			},
			values:  map[string]string{"SERVICE": "my-app"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pb.InterpolatePath(tt.values)
			if (err != nil) != tt.wantErr {
				t.Errorf("InterpolatePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("InterpolatePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathBuilder_GenerateAllPaths(t *testing.T) {
	tests := []struct {
		name    string
		pb      *PathBuilder
		want    []PathCombination
		wantErr bool
	}{
		{
			name: "single variable single value",
			pb: &PathBuilder{
				Template: "/path/$SERVICE/env",
				Variables: map[string][]string{
					"SERVICE": {"my-app"},
				},
			},
			want: []PathCombination{
				{
					Path:       "/path/my-app/env",
					Values:     map[string]string{"SERVICE": "my-app"},
					OverlayKey: "my-app",
				},
			},
			wantErr: false,
		},
		{
			name: "single variable multiple values",
			pb: &PathBuilder{
				Template: "/path/$ENV",
				Variables: map[string][]string{
					"ENV": {"stg", "prod"},
				},
			},
			want: []PathCombination{
				{
					Path:       "/path/stg",
					Values:     map[string]string{"ENV": "stg"},
					OverlayKey: "stg",
				},
				{
					Path:       "/path/prod",
					Values:     map[string]string{"ENV": "prod"},
					OverlayKey: "prod",
				},
			},
			wantErr: false,
		},
		{
			name: "multiple variables - cartesian product",
			pb: &PathBuilder{
				Template: "/path/$CLUSTER/$ENV",
				Variables: map[string][]string{
					"CLUSTER": {"alpha", "beta"},
					"ENV":     {"stg", "prod"},
				},
			},
			want: []PathCombination{
				{Path: "/path/alpha/stg", Values: map[string]string{"CLUSTER": "alpha", "ENV": "stg"}, OverlayKey: "alpha/stg"},
				{Path: "/path/alpha/prod", Values: map[string]string{"CLUSTER": "alpha", "ENV": "prod"}, OverlayKey: "alpha/prod"},
				{Path: "/path/beta/stg", Values: map[string]string{"CLUSTER": "beta", "ENV": "stg"}, OverlayKey: "beta/stg"},
				{Path: "/path/beta/prod", Values: map[string]string{"CLUSTER": "beta", "ENV": "prod"}, OverlayKey: "beta/prod"},
			},
			wantErr: false,
		},
		{
			name: "no variables",
			pb: &PathBuilder{
				Template:  "/path/to/static",
				Variables: map[string][]string{},
			},
			want: []PathCombination{
				{
					Path:       "/path/to/static",
					Values:     map[string]string{},
					OverlayKey: "/path/to/static",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pb.GenerateAllPaths()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateAllPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Sort both slices by Path for comparison
			sort.Slice(got, func(i, j int) bool { return got[i].Path < got[j].Path })
			sort.Slice(tt.want, func(i, j int) bool { return tt.want[i].Path < tt.want[j].Path })

			if len(got) != len(tt.want) {
				t.Errorf("GenerateAllPaths() returned %d paths, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].Path != tt.want[i].Path {
					t.Errorf("GenerateAllPaths()[%d].Path = %v, want %v", i, got[i].Path, tt.want[i].Path)
				}
				if got[i].OverlayKey != tt.want[i].OverlayKey {
					t.Errorf("GenerateAllPaths()[%d].OverlayKey = %v, want %v", i, got[i].OverlayKey, tt.want[i].OverlayKey)
				}
				if !reflect.DeepEqual(got[i].Values, tt.want[i].Values) {
					t.Errorf("GenerateAllPaths()[%d].Values = %v, want %v", i, got[i].Values, tt.want[i].Values)
				}
			}
		})
	}
}

func TestNewPathBuilder(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		valuesStr string
		wantErr   bool
	}{
		{
			name:      "valid inputs",
			template:  "/path/$SERVICE/$ENV",
			valuesStr: "SERVICE=my-app;ENV=stg,prod",
			wantErr:   false,
		},
		{
			name:      "invalid values string",
			template:  "/path/$SERVICE",
			valuesStr: "INVALID",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := NewPathBuilder(tt.template, tt.valuesStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPathBuilder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && pb == nil {
				t.Error("NewPathBuilder() returned nil PathBuilder")
			}
		})
	}
}

func TestPathBuilder_GetRelativePaths(t *testing.T) {
	pb := &PathBuilder{
		Template: "/services/$SERVICE/clusters/$CLUSTER/$ENV",
		Variables: map[string][]string{
			"SERVICE": {"my-app"},
			"CLUSTER": {"alpha"},
			"ENV":     {"stg", "prod"},
		},
	}

	paths, err := pb.GetRelativePaths()
	if err != nil {
		t.Errorf("GetRelativePaths() error = %v", err)
		return
	}

	if len(paths) != 2 {
		t.Errorf("GetRelativePaths() returned %d paths, want 2", len(paths))
	}

	sort.Strings(paths)
	expected := []string{
		"/services/my-app/clusters/alpha/prod",
		"/services/my-app/clusters/alpha/stg",
	}
	sort.Strings(expected)

	if !reflect.DeepEqual(paths, expected) {
		t.Errorf("GetRelativePaths() = %v, want %v", paths, expected)
	}
}

