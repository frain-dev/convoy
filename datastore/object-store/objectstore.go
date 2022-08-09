package objectstore

type ObjectStore interface {
	Save(string) error
}

type ObjectStoreOptions struct {
	Bucket           string
	AccessKey        string
	SecretKey        string
	Region           string
	SessionToken     string
	OnPremStorageDir string
}
