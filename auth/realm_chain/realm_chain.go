package realm_chain

import (
	"errors"
	"fmt"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/file"
)

type chainMap map[string]auth.Realm

var (
	realmChainSingleton atomic.Value
	ErrAuthFailed       = errors.New("no realm could authenticate these credentials")
	ErrNilRealm         = errors.New("registering a nil realm is not allowed")
)

// RealmChain represents a group of realms to be called for authentication.
// When RealmChain.Authenticate is called, the Authenticate method of all
// registered realms is called. If at least one realm can authenticate the
// given auth.Credential, RealmChain.Authenticate will not return an error
type RealmChain struct {
	chain chainMap
}

func Get() (*RealmChain, error) {
	rc, ok := realmChainSingleton.Load().(*RealmChain)
	if !ok {
		return &RealmChain{}, errors.New("call Init before this function")
	}
	return rc, nil
}

func Init(opts ...auth.RealmOption) error {
	rc := newRealmChain()
	for _, opt := range opts {
		realmType := auth.RealmType(opt.Type)
		switch realmType {
		case auth.RealmTypeFile:
			fr, err := file.NewFileRealm(opt.Path)
			if err != nil {
				return fmt.Errorf("failed to initialize file realm '%s': %v", opt.Name, err)
			}

			fr.Name = opt.Name
			err = rc.RegisterRealm(fr)
			if err != nil {
				return fmt.Errorf("failed to register file realm in realm chain: %v", err)
			}
		}
	}

	realmChainSingleton.Store(rc)
	return nil
}

// Authenticate calls the Authenticate method of all registered realms.
// If at least one realm can authenticate the given auth.Credential, Authenticate will not return an error
func (rc *RealmChain) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	var err error
	var authUser *auth.AuthenticatedUser

	for name, realm := range rc.chain {
		authUser, err = realm.Authenticate(cred)
		if err == nil {
			return authUser, nil
		}
		log.WithError(err).Errorf("realm %s failed to authenticate cred: %+v", name, cred)
	}
	return nil, ErrAuthFailed
}

func newRealmChain() *RealmChain {
	return &RealmChain{chain: chainMap{}}
}

func (rc *RealmChain) RegisterRealm(r auth.Realm) error {
	if r == nil {
		return ErrNilRealm
	}

	name := r.GetName()
	_, ok := rc.chain[name]
	if ok {
		return fmt.Errorf("a realm with the name '%s' has already been registered", name)
	}
	rc.chain[name] = r

	return nil
}
