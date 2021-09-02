package util

import "testing"

func Test_computeJSONHmac(t *testing.T) {
	type args struct {
		secret string
		data   string
		order  bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "order sample ordered payload - same signature",
			args: args{
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  true,
			},
			want:    "2ecc66d5f574a10373d1b8d4a390b5c469f2a8079ef9945705026ad8bcc5482a",
			wantErr: false,
		},
		{
			name: "order sample unordered payload - 1 - same signature",
			args: args{
				secret: "my-long-secret",
				data:   `{"b": {}, "e": "123", "a": 1}`,
				order:  true,
			},
			want:    "2ecc66d5f574a10373d1b8d4a390b5c469f2a8079ef9945705026ad8bcc5482a",
			wantErr: false,
		},
		{
			name: "order sample unordered payload - 2 - same signature",
			args: args{
				secret: "my-long-secret",
				data:   `{"e": "123", "a": 1,"b": {}     }`,
				order:  true,
			},
			want:    "2ecc66d5f574a10373d1b8d4a390b5c469f2a8079ef9945705026ad8bcc5482a",
			wantErr: false,
		},
		{
			name: "sample invalid payload",
			args: args{
				secret: "my-long-secret",
				data:   `{"e": "123", "a": 1,"b": {}     \}`,
				order:  true,
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "do not order sample ordered payload - different signature",
			args: args{
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "cb212947c469e3943b1c516415bbd5284652fa1049625abc76cbb423e021b257",
			wantErr: false,
		},
		{
			name: "do not order sample unordered payload - 1 - different signature",
			args: args{
				secret: "my-long-secret",
				data:   `{"b": {}, "e": "123", "a": 1}`,
				order:  false,
			},
			want:    "d95e5c838befbdb71d61d9c6cb33f7dfcd4107ad74de27f4fbbc460befb38289",
			wantErr: false,
		},
		{
			name: "do not order sample unordered payload - 2 - different signature",
			args: args{
				secret: "my-long-secret",
				data:   `{"e": "123", "a": 1,"b": {}     }`,
				order:  false,
			},
			want:    "8c5e234215cc4bd48e806fee4db00cc3ff0f7e5dab1383fdc77363286c6c5909",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeJSONHmac(tt.args.secret, tt.args.data, tt.args.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeJSONHmac() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ComputeJSONHmac() got = %v, want %v", got, tt.want)
			}
		})
	}
}
