package realm_chain

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/file"
	"github.com/frain-dev/convoy/auth/realm/noop"
	"github.com/frain-dev/convoy/config"
	log "github.com/sirupsen/logrus"
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

func Init(authConfig *config.AuthConfiguration) error {
	rc := newRealmChain()

	// validate authentication realms
	if authConfig.RequireAuth {
		fr, err := file.NewFileRealm(&authConfig.File)
		if err != nil {
			return err
		}

		err = rc.RegisterRealm(fr)
		if err != nil {
			return errors.New("failed to register file realm in realm chain")
		}
	} else {
		log.Warnf("using noop realm for authentication: all requests will be authenticated with super_user role")
		err := rc.RegisterRealm(noop.NewNoopRealm())
		if err != nil {
			return errors.New("failed to register noop realm in realm chain")
		}
	}

	realmChainSingleton.Store(rc)
	return nil
}

// Authenticate calls the Authenticate method of all registered realms.
// If at least one realm can authenticate the given auth.Credential, Authenticate will not return an error
func (rc *RealmChain) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	var err error
	var authUser *auth.AuthenticatedUser

	for name, realm := range rc.chain {
		authUser, err = realm.Authenticate(ctx, cred)
		if err == nil {
			return authUser, nil
		}
		// TODO(daniel): starting to think logging cred itself doesn't add any value
		log.WithError(err).Errorf("realm %s failed to authenticate cred: %s", name, cred)
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
