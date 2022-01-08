package memberlist

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/memberlist"
	log "github.com/sirupsen/logrus"
)

var (
	mtx        sync.RWMutex
	broadcasts *memberlist.TransmitLimitedQueue
)

type delegate struct{}

type update struct {
	Action string // kill
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(b []byte) {
	if len(b) == 0 {
		return
	}

	switch b[0] {
	case 'd': // data
		var updates []*update
		if err := json.Unmarshal(b[1:], &updates); err != nil {
			return
		}
		mtx.Lock()
		for _, u := range updates {
			switch u.Action {
			case "kill":
				os.Exit(0)
			}
		}
		mtx.Unlock()
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return broadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	mtx.RLock()
	m, _ := json.Marshal([]*update{})
	mtx.RUnlock()
	b, _ := json.Marshal(m)
	return b
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
}

type eventDelegate struct{}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	log.Infof("A node has joined: " + node.String())
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	log.Infof("A node has left: " + node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	log.Infof("A node was updated: " + node.String())
}

//Create a memberlist or join one if a member is specified.
func CreateMemberlist(members string, hostname string) error {
	c := memberlist.DefaultLocalConfig()
	c.Events = &eventDelegate{}
	c.Delegate = &delegate{}
	c.BindPort = 0
	c.Name = hostname
	m, err := memberlist.Create(c)
	if err != nil {
		return err
	}
	if len(members) > 0 {
		parts := strings.Split(members, ",")
		_, err := m.Join(parts)
		if err != nil {
			return err
		}
	}
	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return m.NumMembers()
		},
		RetransmitMult: 3,
	}
	node := m.LocalNode()
	log.Infof("Local member %s:%d\n", node.Addr, node.Port)
	return nil
}
