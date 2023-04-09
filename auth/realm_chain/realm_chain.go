package realm_chain

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/file"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/auth/realm/native"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
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

func Init(authConfig *config.AuthConfiguration, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, cache cache.Cache) error {
	rc := newRealmChain()

	// validate authentication realms
	fr, err := file.NewFileRealm(&authConfig.File)
	if err != nil {
		return err
	}

	err = rc.RegisterRealm(fr)
	if err != nil {
		return errors.New("failed to register file realm in realm chain")
	}

	if authConfig.Native.Enabled {
		nr := native.NewNativeRealm(apiKeyRepo, userRepo)
		err = rc.RegisterRealm(nr)
		if err != nil {
			return errors.New("failed to register native realm in realm chain")
		}
	}

	if authConfig.Jwt.Enabled {
		jr := jwt.NewJwtRealm(userRepo, &authConfig.Jwt, cache)
		err = rc.RegisterRealm(jr)
		if err != nil {
			return errors.New("failed to register jwt realm in realm chain")
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

	for _, realm := range rc.chain {
		authUser, err = realm.Authenticate(ctx, cred)
		if err == nil {
			return authUser, nil
		}
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
