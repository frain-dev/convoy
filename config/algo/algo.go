package algo

const (
	MD5        string = "MD5"
	SHA1       string = "SHA1"
	SHA224     string = "SHA224"
	SHA256     string = "SHA256"
	SHA384     string = "SHA384"
	SHA512     string = "SHA512"
	SHA3_224   string = "SHA3_224"
	SHA3_256   string = "SHA3_256"
	SHA3_384   string = "SHA3_384"
	SHA3_512   string = "SHA3_512"
	SHA512_224 string = "SHA512_224"
	SHA512_256 string = "SHA512_256"
)

var Algos []string = []string{
	MD5, SHA1, SHA224,
	SHA256, SHA384, SHA512,
	SHA3_224, SHA3_256, SHA3_384,
	SHA3_512, SHA512_224, SHA512_256,
}

var M = map[string]string{
	MD5:        MD5,
	SHA1:       SHA1,
	SHA224:     SHA224,
	SHA256:     SHA256,
	SHA384:     SHA384,
	SHA512:     SHA512,
	SHA3_224:   SHA3_224,
	SHA3_256:   SHA3_256,
	SHA3_384:   SHA3_384,
	SHA3_512:   SHA3_512,
	SHA512_224: SHA512_224,
	SHA512_256: SHA512_256,
}
