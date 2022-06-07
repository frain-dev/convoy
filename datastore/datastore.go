package datastore

import (
	"context"
)

type DatabaseClient interface {
	GetName() string
	Client() interface{}
	Disconnect(context.Context) error

	APIRepo() APIKeyRepository
	GroupRepo() GroupRepository
	EventRepo() EventRepository
	AppRepo() ApplicationRepository
	SubRepo() SubscriptionRepository
	EventDeliveryRepo() EventDeliveryRepository
	SourceRepo() SourceRepository
	OrganisationRepo() OrganisationRepository
	OrganisationMemberRepo() OrganisationMemberRepository
	OrganisationInviteRepo() OrganisationInviteRepository
	UserRepo() UserRepository
}
