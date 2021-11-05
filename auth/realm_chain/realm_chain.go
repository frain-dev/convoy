package realm_chain

import (
	"errors"
	"fmt"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy/auth"
)

var (
	ErrCastFailed = errors.New("failed to cast realm chain map")
	ErrAuthFailed = errors.New("no realm could authenticate these credentials")
	ErrNilRealm   = errors.New("registering a nil realm is not allowed")
)

// RealmChain represents a group of realms to be called for authentication.
// When RealmChain.Authenticate is called, the Authenticate method of all
// registered realms is called. If at least one realm can authenticate the
// given auth.Credential, RealmChain.Authenticate will not return an error
type RealmChain struct {
	chain atomic.Value
}

func Get() *RealmChain {
	return rc
}

// Authenticate calls the Authenticate method of all registered realms.
// If at least one realm can authenticate the given auth.Credential, Authenticate will not return an error
func (rc *RealmChain) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	chain, ok := rc.chain.Load().(chainMap)
	if !ok {
		return nil, ErrCastFailed
	}

	var err error
	var authUser *auth.AuthenticatedUser

	for name, realm := range chain {
		authUser, err = realm.Authenticate(cred)
		if err == nil {
			return authUser, nil
		}
		log.WithError(err).Errorf("realm %s failed to authenticate cred: %+v", name, cred)
	}
	return nil, ErrAuthFailed
}

var rc = newRealmChain()

type chainMap map[string]auth.Realm

func newRealmChain() *RealmChain {
	rc := &RealmChain{}
	rc.chain.Store(chainMap{})
	return rc
}

func (rc *RealmChain) RegisterRealm(r auth.Realm) error {
	if r == nil {
		return ErrNilRealm
	}

	chain, ok := rc.chain.Load().(chainMap)
	if !ok {
		return ErrCastFailed
	}

	name := r.GetName()
	_, ok = chain[name]
	if ok {
		return fmt.Errorf("a realm with the name '%s' has already been registered", name)
	}
	chain[name] = r

	rc.chain.Store(chain)

	return nil
}
