package worker

import (
	"context"
	"log"

	"github.com/frain-dev/convoy/datastore"
)

func NewGroupTask(groupRepo datastore.GroupRepository, filter *datastore.GroupFilter) {
	go func() {
		groups, err := groupRepo.LoadGroups(context.TODO(), filter)
		if err != nil {
			log.Fatalf("an error occurred while fetching Groups:", err)
			return
		}
	}()
}
