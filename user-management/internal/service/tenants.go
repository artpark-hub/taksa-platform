package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "user-management/api/tenants/v1"
	"user-management/internal/biz"
	"user-management/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type TenantsService struct {
	v1.UnimplementedTenantsServiceServer

	uc              *biz.TenantsUsecase
	kratosPublicURL string
	log             *log.Helper
}

// JWTDetails holds all the information extracted from JWT
type JWTDetails struct {
	OrganizationName string
	Role             string
	Email            string
	Subject          string
	// Add more fields here as needed in the future
	// FirstName        string
	// LastName         string
}

func NewTenantsService(uc *biz.TenantsUsecase, d *data.Data, logger log.Logger) *TenantsService {
	return &TenantsService{
		uc:              uc,
		kratosPublicURL: d.KratosPublicURL(),
		log:             log.NewHelper(logger),
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
		Data: &v1.TenantData{
			IdentityId:       data.IdentityID,
			OrganizationName: data.OrganizationName,
			Status:           "active",
		},
	}, nil
}

// ---------------------------------------------------------
// 2. LoginUser - Proxy to Kratos API flow
// ---------------------------------------------------------
func (s *TenantsService) LoginUser(ctx context.Context, req *v1.LoginUserRequest) (*v1.LoginUserResponse, error) {
	s.log.WithContext(ctx).Infof("LoginUser: %s", req.Email)

	// Step 1: Get login flow from Kratos
	flowURL := fmt.Sprintf("%s/self-service/login/api", s.kratosPublicURL)
	flowResp, err := http.Get(flowURL)
	if err != nil {
		s.log.Errorf("Failed to get login flow: %v", err)
		return nil, fmt.Errorf("failed to initialize login flow: %w", err)
	}
	defer flowResp.Body.Close()

	if flowResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(flowResp.Body)
		s.log.Errorf("Kratos flow init failed: %s", string(body))
		return nil, fmt.Errorf("kratos flow init failed with status %d", flowResp.StatusCode)
	}

	var flow struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(flowResp.Body).Decode(&flow); err != nil {
		s.log.Errorf("Failed to decode flow: %v", err)
		return nil, fmt.Errorf("failed to decode flow response: %w", err)
	}

	s.log.Infof("Login flow ID: %s", flow.ID)

	// Step 2: Submit login credentials
	loginData := map[string]interface{}{
		"method":     "password",
		"identifier": req.Email,
		"password":   req.Password,
	}

	loginJSON, err := json.Marshal(loginData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login data: %w", err)
	}

	loginURL := fmt.Sprintf("%s/self-service/login?flow=%s", s.kratosPublicURL, flow.ID)
	loginResp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		s.log.Errorf("Failed to submit login: %v", err)
		return nil, fmt.Errorf("failed to submit login: %w", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(loginResp.Body)
		s.log.Errorf("Kratos login failed: %s", string(body))
		return nil, fmt.Errorf("login failed: invalid credentials or error")
	}

	// Parse the FULL response to get session and identity
	var kratosLogin struct {
		SessionToken string `json:"session_token"`
		Session      struct {
			Identity struct {
				Traits map[string]interface{} `json:"traits"`
			} `json:"identity"`
		} `json:"session"`
	}

	if err := json.NewDecoder(loginResp.Body).Decode(&kratosLogin); err != nil {
		s.log.Errorf("Failed to decode login response: %v", err)
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	if kratosLogin.SessionToken == "" {
		s.log.Error("No session token in response")
		return nil, fmt.Errorf("no session token received from Kratos")
	}

	// Extract user information from traits
	traits := kratosLogin.Session.Identity.Traits

	email, _ := traits["email"].(string)
	orgName, _ := traits["organization_name"].(string)
	role, _ := traits["role"].(string)

	// Extract name fields
	var firstName, lastName string
	if nameMap, ok := traits["name"].(map[string]interface{}); ok {
		firstName, _ = nameMap["first"].(string)
		lastName, _ = nameMap["last"].(string)
	}

	s.log.Infof("Login successful for user: %s (%s %s) - Org: %s, Role: %s", email, firstName, lastName, orgName, role)

	return &v1.LoginUserResponse{
		SessionToken: kratosLogin.SessionToken,
		User: &v1.UserInfo{
			Email:            email,
			FirstName:        firstName,
			LastName:         lastName,
			OrganizationName: orgName,
			Role:             role,
		},
	}, nil
}

// ---------------------------------------------------------
// 3. GetJWTToken
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
		}, nil
	}

	return nil, errors.New("failed to extract transport info")
}

// ---------------------------------------------------------
// 4. CreateSubUser - ONLY MASTERS CAN CREATE
// ---------------------------------------------------------
func (s *TenantsService) CreateSubUser(ctx context.Context, req *v1.CreateSubUserRequest) (*v1.CreateSubUserResponse, error) {
	// Get details from JWT
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, err
	}

	// Check if user is a master
	if details.Role != "master" {
		s.log.Warnf("Unauthorized create sub-user attempt by role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can create sub-users")
	}

	s.log.WithContext(ctx).Infof("CreateSubUser: %s under Org: %s by master user", req.Email, details.OrganizationName)

	data, err := s.uc.CreateSubUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, details.OrganizationName)
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

// ---------------------------------------------------------
// 5. ListSubUsers
// ---------------------------------------------------------
func (s *TenantsService) ListSubUsers(ctx context.Context, req *v1.ListSubUsersRequest) (*v1.ListSubUsersResponse, error) {
	// Get details from JWT
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

// ---------------------------------------------------------
// 6. DeleteSubUser - ONLY MASTERS CAN DELETE
// ---------------------------------------------------------
func (s *TenantsService) DeleteSubUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	// Get details from JWT
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	// Check if user is a master
	if details.Role != "master" {
		s.log.Warnf("Unauthorized delete attempt by non-master user with role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can delete other users")
	}

	s.log.Infof("Master user deleting sub-user: %s", req.IdentityId)

	err = s.uc.DeleteSubUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "User deleted successfully",
	}, nil
}

// ---------------------------------------------------------
// 7. DeleteMasterUser - ONLY MASTERS CAN DELETE
// ---------------------------------------------------------
func (s *TenantsService) DeleteMasterUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	// Get details from JWT
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	// Check if user is a master
	if details.Role != "master" {
		s.log.Warnf("Unauthorized delete master user attempt by role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can delete other users")
	}

	s.log.Infof("Master user deleting master user: %s", req.IdentityId)

	err = s.uc.DeleteMasterUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "Master user deleted successfully",
	}, nil
}

// ---------------------------------------------------------
// Helper: Extract All Details from JWT (Single Source of Truth)
// ---------------------------------------------------------
func (s *TenantsService) getDetailsFromJWT(ctx context.Context) (*JWTDetails, error) {
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
		return nil, errors.New("jwt token missing from headers")
	}

	// 2. Parse the token (WITHOUT verifying signature, since Oathkeeper did it)
	token, _, err := new(jwtv5.Parser).ParseUnverified(tokenString, jwtv5.MapClaims{})
	if err != nil {
		s.log.Errorf("Token Parsing Failed: %v", err)
		return nil, errors.New("failed to parse jwt token")
	}

	// 3. Extract all claims into JWTDetails struct
	details := &JWTDetails{}

	if claims, ok := token.Claims.(jwtv5.MapClaims); ok {
		// Extract from root level
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

		// Fallback: Check inside 'traits' object
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

	// Validate required fields
	if details.OrganizationName == "" {
		return nil, errors.New("organization_name claim not found in token")
	}
	if details.Role == "" {
		return nil, errors.New("role claim not found in token")
	}

	return details, nil
}
