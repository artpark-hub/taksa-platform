package service

import (
	"context"
	"errors"
	"strings"

	v1 "user-management/api/tenants/v1"
	"user-management/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Renamed to match "Tenants" migration
type TenantsService struct {
	v1.UnimplementedTenantsServiceServer

	uc  *biz.TenantsUsecase
	log *log.Helper
}

// Constructor must match what wire.go expects (NewTenantsService)
func NewTenantsService(uc *biz.TenantsUsecase, logger log.Logger) *TenantsService {
	return &TenantsService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// ---------------------------------------------------------
// 1. RegisterMasterUser
// ---------------------------------------------------------
func (s *TenantsService) RegisterMasterUser(ctx context.Context, req *v1.RegisterMasterUserRequest) (*v1.RegisterMasterUserResponse, error) {
	s.log.WithContext(ctx).Infof("RegisterMasterUser: %s (Org: %s)", req.Email, req.OrganizationName)

	data, err := s.uc.RegisterMasterUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, req.OrganizationName)
	if err != nil {
		return nil, err
	}

	return &v1.RegisterMasterUserResponse{
		Data: &v1.TenantData{ // Changed from OrganizationData to TenantData
			IdentityId:       data.IdentityID,
			OrganizationName: data.OrganizationName,
			Status:           "active", // data.Status might be empty if not passed back, defaulting to active
		},
		Meta: &v1.ResponseMeta{Timestamp: "now"},
	}, nil
}

// ---------------------------------------------------------
// 2. GetJWTToken
// ---------------------------------------------------------
func (s *TenantsService) GetJWTToken(ctx context.Context, req *v1.GetJWTTokenRequest) (*v1.GetJWTTokenResponse, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")

		// Strip "Bearer " prefix if present to return pure token
		token := authHeader
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			token = authHeader[7:]
		}

		if token == "" {
			return nil, errors.New("authorization header (JWT) missing from Oathkeeper")
		}

		return &v1.GetJWTTokenResponse{
			Data: &v1.TokenData{
				JwtToken: token,
			},
			Meta: &v1.ResponseMeta{Timestamp: "now"},
		}, nil
	}

	return nil, errors.New("failed to extract transport info")
}

// ---------------------------------------------------------
// 3. CreateSubUser
// ---------------------------------------------------------
func (s *TenantsService) CreateSubUser(ctx context.Context, req *v1.CreateSubUserRequest) (*v1.CreateSubUserResponse, error) {
	orgName, err := s.getOrgFromContext(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract org from JWT: %v", err)
		return nil, err
	}

	s.log.WithContext(ctx).Infof("CreateSubUser: %s under Org: %s", req.Email, orgName)

	data, err := s.uc.CreateSubUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, orgName)
	if err != nil {
		return nil, err
	}

	return &v1.CreateSubUserResponse{
		Data: &v1.UserData{
			IdentityId:       data.IdentityID,
			OrganizationName: data.OrganizationName,
		},
		Meta: &v1.ResponseMeta{Timestamp: "now"},
	}, nil
}

// ---------------------------------------------------------
// 4. ListSubUsers
// ---------------------------------------------------------
func (s *TenantsService) ListSubUsers(ctx context.Context, req *v1.ListSubUsersRequest) (*v1.ListSubUsersResponse, error) {
	orgName, err := s.getOrgFromContext(ctx)
	if err != nil {
		return nil, err
	}

	users, err := s.uc.ListSubUsers(ctx, orgName)
	if err != nil {
		return nil, err
	}

	var protoUsers []*v1.UserSummary
	for _, u := range users {
		protoUsers = append(protoUsers, &v1.UserSummary{
			IdentityId:       u.IdentityID,
			Email:            u.Email,
			FirstName:        u.FirstName,
			LastName:         u.LastName,
			OrganizationName: u.OrganizationName,
		})
	}

	return &v1.ListSubUsersResponse{
		Users: protoUsers,
		Meta:  &v1.ResponseMeta{Timestamp: "now"},
	}, nil
}

// ---------------------------------------------------------
// 5. DeleteSubUser
// ---------------------------------------------------------
func (s *TenantsService) DeleteSubUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	err := s.uc.DeleteSubUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "User deleted successfully",
		Meta:    &v1.ResponseMeta{Timestamp: "now"},
	}, nil
}

// ---------------------------------------------------------
// 6. DeleteMasterUser
// ---------------------------------------------------------
func (s *TenantsService) DeleteMasterUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	err := s.uc.DeleteMasterUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "Master user deleted successfully",
		Meta:    &v1.ResponseMeta{Timestamp: "now"},
	}, nil
}

// ---------------------------------------------------------
// Helper: Extract Organization from JWT
// ---------------------------------------------------------
func (s *TenantsService) getOrgFromContext(ctx context.Context) (string, error) {
	// 1. Get the token string from the Authorization header
	var tokenString string
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			tokenString = authHeader[7:]
		} else {
			tokenString = authHeader
		}
	}

	// Clean up whitespace/quotes
	tokenString = strings.TrimSpace(tokenString)
	tokenString = strings.Trim(tokenString, "\"")

	if tokenString == "" {
		return "", errors.New("jwt token missing from headers")
	}

	// 2. Parse the token (WITHOUT verifying signature, since Oathkeeper did it)
	token, _, err := new(jwtv5.Parser).ParseUnverified(tokenString, jwtv5.MapClaims{})
	if err != nil {
		s.log.Errorf("Token Parsing Failed: %v", err)
		return "", errors.New("failed to parse jwt token")
	}

	// 3. Extract Claims
	if claims, ok := token.Claims.(jwtv5.MapClaims); ok {
		// Look for 'organization_name' (root level)
		if org, found := claims["organization_name"]; found {
			return org.(string), nil
		}

		// Fallback: Check inside 'traits' object
		if traits, found := claims["traits"].(map[string]interface{}); found {
			if org, ok := traits["organization_name"].(string); ok {
				return org, nil
			}
		}
	}

	return "", errors.New("organization_name claim not found in token")
}
