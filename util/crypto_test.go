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
		{
			name: "test timestamp data - same signature",
			args: args{
				secret: "my-long-secret",
				data:   "1647445058",
				order:  false,
			},
			want:    "e9157602697fb18eaa538b8c4e8e10719ded19d12b4b825905173f6a3bf9eb55",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeJSONHmac("SHA256", tt.args.data, tt.args.secret, tt.args.order)
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

func Test_computeJSONHmac_DifferentHashes(t *testing.T) {
	type args struct {
		hash   string
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
			name: "SHA512_224 - payload",
			args: args{
				hash:   "SHA512_224",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "4f7bab1567a118dca99bbe39c1ca39dc6563003258f27be2098984cd",
			wantErr: false,
		},
		{
			name: "SHA512_256 - payload",
			args: args{
				hash:   "SHA512_256",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "9a248c90f73f45396f8045cbea018cb127279009372b4c7f0ddaad533a29f62b",
			wantErr: false,
		},
		{
			name: "SHA1 - payload",
			args: args{
				hash:   "SHA1",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "2bd70a39cc120624d0b9468bbd44ddf67a5c57a0",
			wantErr: false,
		},
		{
			name: "SHA3_224 - payload",
			args: args{
				hash:   "SHA3_224",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "f05607475c05e85e8fb7e5a70c3ef5d86391ba4dc87ea41d97f8f74d",
			wantErr: false,
		},
		{
			name: "SHA3_256 - payload",
			args: args{
				hash:   "SHA3_256",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "b3f8d48c905f97b99087a9f8a72b2d7df499199c6a978901a9f71e0180d2e414",
			wantErr: false,
		},
		{
			name: "SHA3_384 - payload",
			args: args{
				hash:   "SHA3_384",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "692a68a8e37517a251b6652ceb439bd2d5502cb1561df99c9a1fec2a8dd020cc84693f4decdce3068773bf81eb72bb50",
			wantErr: false,
		},
		{
			name: "SHA256 - payload",
			args: args{
				hash:   "SHA256",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "cb212947c469e3943b1c516415bbd5284652fa1049625abc76cbb423e021b257",
			wantErr: false,
		},
		{
			name: "SHA512 - payload",
			args: args{
				hash:   "SHA512",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "c0089311dd758cbb1161cf94e2266a35a9ef9e1ddd092cb938fc61629d619d3ec5d299f9bb8b3d599344b2c1c3fe6a3eea8dcda3a56747e227919f4776311790",
			wantErr: false,
		},
		{
			name: "SHA384 - payload",
			args: args{
				hash:   "SHA384",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "40ca9a3bd0b93da5d7ff7bf55bd00a3c632863ec53c38016e5ee2d4b1853bce8ed36e9d457f4061a12f163ba0ae8c0d9",
			wantErr: false,
		},
		{
			name: "SHA3_512 - payload",
			args: args{
				hash:   "SHA3_512",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "4b4bad64e8d8093e8bf780a654107780ffaa200267ea59ee7649b49f3d7f9bb26026164468b57dac11c54f9aeeeb015bd5e4678e0b8d009f390cc5aa43148b28",
			wantErr: false,
		},
		{
			name: "MD5 - payload",
			args: args{
				hash:   "MD5",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "be2bcc264914982c42858b43c9d39243",
			wantErr: false,
		},
		{
			name: "SHA224 - payload",
			args: args{
				hash:   "SHA224",
				secret: "my-long-secret",
				data:   `{"a": 1, "b": {}, "e": "123"}`,
				order:  false,
			},
			want:    "6e3112d3c44119be161d206b7d5faf2594b8bb82f68c1c5757357176",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeJSONHmac(tt.args.hash, tt.args.data, tt.args.secret, tt.args.order)
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
