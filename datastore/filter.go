package datastore

type Filter struct {
	Query        string
	Group        *Group
	AppID        string
	EventID      string
	Pageable     Pageable
	Status       []EventDeliveryStatus
	SearchParams SearchParams
}

type SourceFilter struct {
	Type     string
	Provider string
}
