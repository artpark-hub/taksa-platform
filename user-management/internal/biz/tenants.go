package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type User struct {
	IdentityID       string
	Email            string
	FirstName        string
	LastName         string
	OrganizationName string
	OrganizationID   string
	Role             string
}

type TenantsRepo interface {
	CreateKratosIdentity(ctx context.Context, email, password, firstName, lastName, orgName, orgID, role string) (*User, error)
	ListIdentitiesByOrg(ctx context.Context, orgName, orgID string) ([]*User, error)
	DeleteIdentity(ctx context.Context, id string) error
	UpdateIdentity(ctx context.Context, id, firstName, lastName, role string) (*User, error)
}

type TenantsUsecase struct {
	repo TenantsRepo
	log  *log.Helper
}

func NewTenantsUsecase(repo TenantsRepo, logger log.Logger) *TenantsUsecase {
	return &TenantsUsecase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

func (uc *TenantsUsecase) CreateSubUser(ctx context.Context, email, password, firstName, lastName, orgName, orgID, role string) (*User, error) {
	return uc.repo.CreateKratosIdentity(ctx, email, password, firstName, lastName, orgName, orgID, role)
}

func (uc *TenantsUsecase) ListSubUsers(ctx context.Context, orgName, orgID string) ([]*User, error) {
	return uc.repo.ListIdentitiesByOrg(ctx, orgName, orgID)
}

func (uc *TenantsUsecase) DeleteSubUser(ctx context.Context, id string) error {
	return uc.repo.DeleteIdentity(ctx, id)
}

func (uc *TenantsUsecase) DeleteMasterUser(ctx context.Context, id string) error {
	return uc.repo.DeleteIdentity(ctx, id)
}

func (uc *TenantsUsecase) DeleteIncompleteOidcUser(ctx context.Context, id string) error {
	return uc.repo.DeleteIdentity(ctx, id)
}

func (uc *TenantsUsecase) UpdateUserProfile(ctx context.Context, id, firstName, lastName, role string) (*User, error) {
	return uc.repo.UpdateIdentity(ctx, id, firstName, lastName, role)
}
