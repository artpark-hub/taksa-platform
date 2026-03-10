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
	Role             string
}

type TenantsRepo interface {
	CreateKratosIdentity(ctx context.Context, email, password, firstName, lastName, orgName, role string) (*User, error)
	ListIdentitiesByOrg(ctx context.Context, orgName string) ([]*User, error)
	DeleteIdentity(ctx context.Context, id string) error
	UpdateIdentity(ctx context.Context, id, firstName, lastName, role string) (*User, error) // UPDATED: removed email
}

type TenantsUsecase struct {
	repo TenantsRepo
	log  *log.Helper
}

func NewTenantsUsecase(repo TenantsRepo, logger log.Logger) *TenantsUsecase {
	return &TenantsUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *TenantsUsecase) RegisterMasterUser(ctx context.Context, email, password, firstName, lastName, orgName string) (*User, error) {
	return uc.repo.CreateKratosIdentity(ctx, email, password, firstName, lastName, orgName, "master")
}

func (uc *TenantsUsecase) CreateSubUser(ctx context.Context, email, password, firstName, lastName, orgName string) (*User, error) {
	return uc.repo.CreateKratosIdentity(ctx, email, password, firstName, lastName, orgName, "sub")
}

func (uc *TenantsUsecase) ListSubUsers(ctx context.Context, orgName string) ([]*User, error) {
	return uc.repo.ListIdentitiesByOrg(ctx, orgName)
}

func (uc *TenantsUsecase) DeleteSubUser(ctx context.Context, id string) error {
	return uc.repo.DeleteIdentity(ctx, id)
}

func (uc *TenantsUsecase) DeleteMasterUser(ctx context.Context, id string) error {
	return uc.repo.DeleteIdentity(ctx, id)
}

func (uc *TenantsUsecase) UpdateUserProfile(ctx context.Context, id, firstName, lastName, role string) (*User, error) {
	return uc.repo.UpdateIdentity(ctx, id, firstName, lastName, role)
}
