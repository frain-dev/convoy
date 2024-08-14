package fflag

import (
	"github.com/frain-dev/convoy/config"
	"reflect"
	"testing"
)

func TestFFlag_CanAccessFeature(t *testing.T) {
	type fields struct {
		Features map[FeatureFlagKey]FeatureFlagState
	}
	type args struct {
		key FeatureFlagKey
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "default state - no prometheus",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     disabled,
					FullTextSearch: enabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: Prometheus,
			},
			want: false,
		},
		{
			name: "default state - search available",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     disabled,
					FullTextSearch: enabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: FullTextSearch,
			},
			want: true,
		},
		{
			name: "all enabled state - prometheus available",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     enabled,
					FullTextSearch: enabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: Prometheus,
			},
			want: true,
		},
		{
			name: "all enabled state - search available",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     enabled,
					FullTextSearch: enabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: FullTextSearch,
			},
			want: true,
		},
		{
			name: "all disabled state - no prometheus",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     disabled,
					FullTextSearch: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: Prometheus,
			},
			want: false,
		},
		{
			name: "all disabled state - no search",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     disabled,
					FullTextSearch: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: FullTextSearch,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FFlag{
				Features: tt.fields.Features,
			}
			if got := c.CanAccessFeature(tt.args.key); got != tt.want {
				t.Errorf("CanAccessFeature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFFlag(t *testing.T) {
	type args struct {
		c *config.Configuration
	}
	tests := []struct {
		name    string
		args    args
		want    *FFlag
		wantErr bool
	}{
		{
			name: "default state",
			args: args{
				&config.Configuration{},
			},
			want: &FFlag{
				Features: DefaultFeaturesState,
			},
			wantErr: false,
		},
		{
			name: "default state - assert all disabled",
			args: args{
				&config.Configuration{},
			},
			want: &FFlag{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     disabled,
					FullTextSearch: disabled,
				},
			},
			wantErr: false,
		},
		{
			name: "enabled state - prometheus only",
			args: args{
				&config.Configuration{
					EnableFeatureFlag: []string{"prometheus"},
				},
			},
			want: &FFlag{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     enabled,
					FullTextSearch: disabled,
				},
			},
			wantErr: false,
		},
		{
			name: "all disabled state - by default",
			args: args{
				&config.Configuration{},
			},
			want: &FFlag{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:     disabled,
					FullTextSearch: disabled,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFFlag(tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewFFlag() got = %v, want %v", got, tt.want)
			}
		})
	}
}
