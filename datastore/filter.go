package datastore

type Filter struct {
	Group        *Group
	AppID        string
	EventID      string
	Pageable     Pageable
	Status       []EventDeliveryStatus
	SearchParams SearchParams
}
