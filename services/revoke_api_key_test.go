package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/datastore"
)

func TestRevokePersonalAPIKeyService_Run(t *testing.T) {
	type fields struct {
		ProjectRepo datastore.ProjectRepository
		UserRepo    datastore.UserRepository
		APIKeyRepo  datastore.APIKeyRepository
		UID         string
		User        *datastore.User
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := &RevokePersonalAPIKeyService{
				ProjectRepo: tt.fields.ProjectRepo,
				UserRepo:    tt.fields.UserRepo,
				APIKeyRepo:  tt.fields.APIKeyRepo,
				UID:         tt.fields.UID,
				User:        tt.fields.User,
			}
			if err := ss.Run(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("RevokePersonalAPIKeyService.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
