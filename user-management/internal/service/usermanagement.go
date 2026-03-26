package service

import (
	"context"
	"errors"
	"strings"

	v1 "user-management/api/tenants/v1"
	"user-management/internal/biz"
	"user-management/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type UserManagementService struct {
	v1.UnimplementedUserManagementServiceServer

	uc   *biz.TenantsUsecase
	log  *log.Helper
	data *data.Data
}

type JWTDetails struct {
	OrganizationName string
	Role             string
	Email            string
	Subject          string
}

func NewUserManagementService(uc *biz.TenantsUsecase, d *data.Data, logger log.Logger) *UserManagementService {
	return &UserManagementService{
		uc:   uc,
		data: d,
		log:  log.NewHelper(logger),
	}
}

func (s *UserManagementService) CreateSubUser(ctx context.Context, req *v1.CreateSubUserRequest) (*v1.CreateSubUserResponse, error) {
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, err
	}

	if details.Role != "master" {
		s.log.Warnf("Unauthorized create user attempt by role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can create users")
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		return nil, errors.New("role is required")
	}

	if role == "master" {
		return nil, errors.New("forbidden: create_sub_user cannot create master users")
	}

	data, err := s.uc.CreateSubUser(
		ctx,
		req.Email,
		req.Password,
		req.FirstName,
		req.LastName,
		details.OrganizationName,
		role,
	)
	if err != nil {
		return nil, err
	}

	return &v1.CreateSubUserResponse{
		Data: &v1.UserData{
			IdentityId:       data.IdentityID,
			OrganizationName: data.OrganizationName,
		},
	}, nil
}

func (s *UserManagementService) ListSubUsers(ctx context.Context, req *v1.ListSubUsersRequest) (*v1.ListSubUsersResponse, error) {
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, err
	}

	users, err := s.uc.ListSubUsers(ctx, details.OrganizationName)
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
			Role:             u.Role,
		})
	}

	return &v1.ListSubUsersResponse{
		Users: protoUsers,
	}, nil
}

func (s *UserManagementService) DeleteSubUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	if details.Role != "master" {
		s.log.Warnf("Unauthorized delete attempt by non-master user with role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can delete other users")
	}

	users, err := s.uc.ListSubUsers(ctx, details.OrganizationName)
	if err != nil {
		s.log.Errorf("Failed to list organization users: %v", err)
		return nil, errors.New("failed to validate user deletion")
	}

	targetFound := false
	targetIsMaster := false

	for _, user := range users {
		if user.IdentityID == req.IdentityId {
			targetFound = true
			if strings.ToLower(user.Role) == "master" {
				targetIsMaster = true
			}
			break
		}
	}

	if !targetFound {
		return nil, errors.New("user not found in your organization")
	}

	if targetIsMaster {
		return nil, errors.New("forbidden: use delete_master_user for master accounts")
	}

	err = s.uc.DeleteSubUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "User deleted successfully",
	}, nil
}

func (s *UserManagementService) DeleteMasterUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	if details.Role != "master" {
		s.log.Warnf("Unauthorized delete master user attempt by role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can delete master users")
	}

	users, err := s.uc.ListSubUsers(ctx, details.OrganizationName)
	if err != nil {
		s.log.Errorf("Failed to list organization users: %v", err)
		return nil, errors.New("failed to validate master-user deletion")
	}

	masterCount := 0
	targetFound := false
	targetIsMaster := false

	for _, user := range users {
		if strings.ToLower(user.Role) == "master" {
			masterCount++
		}
		if user.IdentityID == req.IdentityId {
			targetFound = true
			if strings.ToLower(user.Role) == "master" {
				targetIsMaster = true
			}
		}
	}

	if !targetFound {
		return nil, errors.New("user not found in your organization")
	}

	if !targetIsMaster {
		return nil, errors.New("the selected user is not a master user")
	}

	if masterCount <= 1 {
		return nil, errors.New("forbidden: an organization must have at least one master user")
	}

	err = s.uc.DeleteMasterUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "Master user deleted successfully",
	}, nil
}

func (s *UserManagementService) UpdateUserProfile(ctx context.Context, req *v1.UpdateUserProfileRequest) (*v1.UpdateUserProfileResponse, error) {
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	isSelf := details.Subject == req.IdentityId
	normalizedCurrentRole := strings.ToLower(strings.TrimSpace(details.Role))
	normalizedReqRole := strings.ToLower(strings.TrimSpace(req.Role))
	isChangingRole := normalizedReqRole != ""

	if normalizedCurrentRole != "master" {
		if !isSelf {
			return nil, errors.New("forbidden: only master users can update other users' profiles")
		}
		if isChangingRole {
			return nil, errors.New("forbidden: you cannot change your role")
		}
	}

	user, err := s.uc.UpdateUserProfile(ctx, req.IdentityId, req.FirstName, req.LastName, normalizedReqRole)
	if err != nil {
		s.log.Errorf("Failed to update user profile: %v", err)
		return nil, err
	}

	return &v1.UpdateUserProfileResponse{
		Status:  "success",
		Message: "User profile updated successfully",
		User: &v1.UserInfo{
			Email:            user.Email,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			OrganizationName: user.OrganizationName,
			Role:             user.Role,
		},
	}, nil
}

func (s *UserManagementService) GetJWTToken(ctx context.Context, req *v1.GetJWTTokenRequest) (*v1.GetJWTTokenResponse, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")

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
		}, nil
	}

	return nil, errors.New("failed to extract transport info")
}

func (s *UserManagementService) getDetailsFromJWT(ctx context.Context) (*JWTDetails, error) {
	var tokenString string

	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			tokenString = authHeader[7:]
		} else {
			tokenString = authHeader
		}
	}

	tokenString = strings.TrimSpace(tokenString)
	tokenString = strings.Trim(tokenString, "\"")

	if tokenString == "" {
		return nil, errors.New("jwt token missing from headers")
	}

	token, _, err := new(jwtv5.Parser).ParseUnverified(tokenString, jwtv5.MapClaims{})
	if err != nil {
		s.log.Errorf("Token Parsing Failed: %v", err)
		return nil, errors.New("failed to parse jwt token")
	}

	details := &JWTDetails{}

	if claims, ok := token.Claims.(jwtv5.MapClaims); ok {
		if org, found := claims["organization_name"]; found {
			details.OrganizationName, _ = org.(string)
		}
		if role, found := claims["role"]; found {
			details.Role, _ = role.(string)
		}
		if email, found := claims["email"]; found {
			details.Email, _ = email.(string)
		}
		if sub, found := claims["sub"]; found {
			details.Subject, _ = sub.(string)
		}

		if traits, found := claims["traits"].(map[string]interface{}); found {
			if details.OrganizationName == "" {
				if org, ok := traits["organization_name"].(string); ok {
					details.OrganizationName = org
				}
			}
			if details.Role == "" {
				if role, ok := traits["role"].(string); ok {
					details.Role = role
				}
			}
			if details.Email == "" {
				if email, ok := traits["email"].(string); ok {
					details.Email = email
				}
			}
		}
	}

	if details.OrganizationName == "" {
		return nil, errors.New("organization_name claim not found in token")
	}
	if details.Role == "" {
		return nil, errors.New("role claim not found in token")
	}

	details.Role = strings.ToLower(details.Role)

	return details, nil
}
