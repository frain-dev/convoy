package objectstore

type ObjectStore interface {
	Save(string) error
}

type ObjectStoreOptions struct {
	Prefix           string
	Bucket           string
	AccessKey        string
	SecretKey        string
	Region           string
	Endpoint         string
	SessionToken     string
	OnPremStorageDir string
}
