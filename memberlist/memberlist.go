package memberlist

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/memberlist"
)

var (
	mtx        sync.RWMutex
	broadcasts *memberlist.TransmitLimitedQueue
)

type broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

type delegate struct{}

type update struct {
	Action string // kill
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
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
	fmt.Println("A node has joined: " + node.String())
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	fmt.Println("A node has left: " + node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	fmt.Println("A node was updated: " + node.String())
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
	log.Printf("Local member %s:%d\n", node.Addr, node.Port)
	return nil
}

func killHandler(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal([]*update{
		{
			Action: "kill",
		},
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	broadcasts.QueueBroadcast(&broadcast{
		msg:    append([]byte("d"), b...),
		notify: nil,
	})
}
