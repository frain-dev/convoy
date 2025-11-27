package migrations

import "context"

// UpdateEndpointOwnerIDKey is a context key used to signal that endpoint owner_ids need to be updated
type UpdateEndpointOwnerIDKey struct{}

type UpdateEndpointOwnerIDValue struct {
	Update      bool
	EndpointIDs []string
}

func SetUpdateEndpointOwnerID(ctx context.Context, update bool, endpointIDs []string) context.Context {
	return context.WithValue(ctx, UpdateEndpointOwnerIDKey{}, UpdateEndpointOwnerIDValue{
		Update:      update,
		EndpointIDs: endpointIDs,
	})
}

func GetUpdateEndpointOwnerID(ctx context.Context) (bool, []string) {
	val, ok := ctx.Value(UpdateEndpointOwnerIDKey{}).(UpdateEndpointOwnerIDValue)
	if !ok {
		return false, nil
	}
	return val.Update, val.EndpointIDs
}
