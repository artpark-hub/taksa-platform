package biz

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/api/common"
	"github.com/artpark-hub/taksa-platform/device-management/internal/pkg/cert"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// DeviceUsecase handles device business logic (registration, listing, updates)
type DeviceUsecase struct {
	store   storage.Store
	authUc  *AuthUsecase
}

// NewDeviceUsecase creates a new device use case
func NewDeviceUsecase(store storage.Store, authUc *AuthUsecase) *DeviceUsecase {
	return &DeviceUsecase{
		store:  store,
		authUc: authUc,
	}
}

// RegisterDevice creates a new device and auth token
func (uc *DeviceUsecase) RegisterDevice(ctx context.Context, req *RegisterDeviceRequest) (*RegisterDeviceResponse, error) {
	// Validate
	if req.CreatedBy == "" || req.Name == "" || req.Location == nil {
		return nil, fmt.Errorf("created_by, name, and location are required")
	}

	// Check if already exists by created_by + name (per-tenant unique)
	existing, _ := uc.store.Devices().GetByCreatedByAndName(ctx, req.CreatedBy, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("device with name '%s' already exists for this tenant", req.Name)
	}

	// Create device with license information
	// Initialize company with default license status if not provided
	company := req.Company
	if company == nil {
		company = &v1.CompanyDetailsExtended{
			Base: &common.CompanyDetails{
				Name: "Default Company",
				LicenseStatus: &common.LicenseStatus{
					IsActive: true,
					ValidTo:  time.Now().AddDate(1, 0, 0).Format(time.RFC3339), // 1 year from now
				},
			},
		}
	} else if company.Base == nil {
		company.Base = &common.CompanyDetails{
			Name: "Default Company",
			LicenseStatus: &common.LicenseStatus{
				IsActive: true,
				ValidTo:  time.Now().AddDate(1, 0, 0).Format(time.RFC3339),
			},
		}
	} else if company.Base.LicenseStatus == nil {
		company.Base.LicenseStatus = &common.LicenseStatus{
			IsActive: true,
			ValidTo:  time.Now().AddDate(1, 0, 0).Format(time.RFC3339),
		}
	} else if company.Base.LicenseStatus.IsActive == false || company.Base.LicenseStatus.ValidTo == "" {
		// Ensure license is active with a default expiry
		if !company.Base.LicenseStatus.IsActive {
			company.Base.LicenseStatus.IsActive = true
		}
		if company.Base.LicenseStatus.ValidTo == "" {
			company.Base.LicenseStatus.ValidTo = time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
		}
	}

	device := &v1.Device{
		Id:        generateUUID(),
		CreatedBy: req.CreatedBy,
		Name:      req.Name,
		Location:  req.Location,
		Company:   company,
		Status:    v1.DeviceStatus_PENDING,
		CreatedAt: timestamppb.Now(),
		LastSeen:  timestamppb.Now(),
		// AuthTokenExpiresAt will be set after token creation
	}

	// Handle certificate: accept provided or generate new one
	var certPEM string
	if req.Certificate != "" {
		// Validate provided certificate
		if err := cert.ValidateCertificatePEM(req.Certificate); err != nil {
			return nil, fmt.Errorf("invalid certificate provided: %w", err)
		}
		certPEM = req.Certificate
	} else {
		// Generate new certificate (key remains device-side)
		var err error
		certPEM, _, err = cert.GenerateCertificateAndKey(device.Id, req.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate certificate: %w", err)
		}
	}

	// Create device certificate entry
	deviceCert := &v2.Certificate{
		UserEmail:   "", // Empty for device certificate (no specific user)
		Certificate: certPEM,
	}

	// Update device with certificate (private key is device-side, not stored)
	device.Certificate = deviceCert.Certificate

	// Save device FIRST (before certificate/token creation so FK constraints are satisfied)
	if err := uc.store.Devices().Save(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to save device: %w", err)
	}

	// Save device certificate AFTER device exists
	if err := uc.store.Certificates().DeviceStore().SaveDevice(ctx, device.Id, deviceCert); err != nil {
		return nil, fmt.Errorf("failed to create device certificate: %w", err)
	}

	// Create auth token AFTER device is saved
	token, err := uc.authUc.CreateAuthToken(ctx, device.Id, 7)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth token: %w", err)
	}

	// Set auth token expiry on device (7 days from now)
	tokenExpiryTime := time.Now().AddDate(0, 0, 7)
	device.AuthTokenExpiresAt = timestamppb.New(tokenExpiryTime)

	// Update device with token expiry timestamp
	if err := uc.store.Devices().Update(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to update device with token expiry: %w", err)
	}

	return &RegisterDeviceResponse{
		Device:       deviceToSummary(device),
		AuthToken:    token,
		Instructions: map[string]string{
			"setup_guide":     "https://docs.example.com/setup-guide",
			"firmware_update": "Download firmware from releases endpoint",
			"config_path":     "/etc/umh-core/config.yaml",
		},
	}, nil
}

// ListDevices retrieves all devices with optional filtering and cursor pagination
// Returns DeviceSummaries (lightweight) instead of full Device objects with certificate data
func (uc *DeviceUsecase) ListDevices(ctx context.Context, filters *storage.DeviceListFilter) ([]*v1.DeviceSummary, error) {
	if filters == nil {
		filters = storage.DefaultDeviceListFilter()
	}

	summaries, err := uc.store.Devices().ListSummaries(ctx, filters)
	if err != nil {
		return nil, err
	}
	return summaries, nil
}

// GetDevice retrieves a device by ID
func (uc *DeviceUsecase) GetDevice(ctx context.Context, deviceID string) (*v1.Device, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is empty")
	}

	return uc.store.Devices().GetByID(ctx, deviceID)
}

// GetDeviceHealth retrieves the latest health metrics for a device
// Returns device's current status and last activity timestamp from the Pull API heartbeat
func (uc *DeviceUsecase) GetDeviceHealth(ctx context.Context, deviceID string) (*GetDeviceHealthResponse, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is empty")
	}

	// Verify device exists and get its current status
	device, err := uc.store.Devices().GetByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Get the latest message (any type) to determine if device is communicating
	// The Pull API heartbeat updates last_seen timestamp regularly
	// The Push API sends action-reply and other messages
	filter := &storage.MessageListFilter{
		DeviceID: deviceID,
		PageSize: 1,
		SortDesc: true, // Most recent first
	}

	messages, _, err := uc.store.Messages().ListHistory(ctx, filter)
	
	// Build response based on device's current state
	resp := &GetDeviceHealthResponse{
		DeviceId:    deviceID,
		LastUpdated: device.LastSeen, // Use device's last_seen from Pull API heartbeat
		Status:      nil,              // Status message not yet implemented
		StatusB64:   "",               // Will be populated when Status message is sent
	}

	// Check if device has recent activity (within last 30 seconds)
	if device.LastSeen != nil {
		lastSeenTime := time.Unix(device.LastSeen.Seconds, int64(device.LastSeen.Nanos))
		timeSinceLastSeen := time.Since(lastSeenTime)
		
		if timeSinceLastSeen < 30*time.Second {
			resp.ErrorMessage = "" // Device is healthy - recently seen
		} else if timeSinceLastSeen < 5*time.Minute {
			resp.ErrorMessage = fmt.Sprintf("device inactive for %v", timeSinceLastSeen.Round(time.Second))
		} else {
			resp.ErrorMessage = fmt.Sprintf("device not seen for %v (may be offline)", timeSinceLastSeen.Round(time.Second))
		}
	} else {
		resp.ErrorMessage = "no Pull API activity recorded (device may be newly registered)"
	}

	// If device has sent messages, extract health data if available
	// Currently looking for status-type messages (future enhancement)
	if len(messages) > 0 && messages[0] != nil {
		msg := messages[0]
		
		// Look for StatusMessage in message content
		// The Content field contains base64-encoded JSON with the message payload
		if msg.Content != "" && msg.Type == "StatusMessage" {
			resp.StatusB64 = msg.Content
			// TODO: Implement proper protobuf unmarshaling of StatusMessage from Content field
		}
	}

	return resp, nil
}

// GetDeviceHealthResponse contains health metrics
type GetDeviceHealthResponse struct {
	DeviceId      string
	LastUpdated   *timestamppb.Timestamp
	Status        *v2.StatusMessage
	StatusB64     string // Base64-encoded StatusMessage fallback
	DeviceTimestamp *timestamppb.Timestamp
	ErrorMessage  string
}

// deviceToSummary converts a full Device to a DeviceSummary (lightweight version)
func deviceToSummary(device *v1.Device) *v1.DeviceSummary {
	if device == nil {
		return nil
	}
	return &v1.DeviceSummary{
		Id:        device.Id,
		CreatedBy: device.CreatedBy,
		Name:      device.Name,
		Location:  device.Location,
		Status:    device.Status,
		CreatedAt: device.CreatedAt,
		LastSeen:  device.LastSeen,
	}
}

// devicesToSummaries converts a slice of Device to DeviceSummaries
func devicesToSummaries(devices []*v1.Device) []*v1.DeviceSummary {
	summaries := make([]*v1.DeviceSummary, len(devices))
	for i, device := range devices {
		summaries[i] = deviceToSummary(device)
	}
	return summaries
}

// UpdateDevice updates device information
func (uc *DeviceUsecase) UpdateDevice(ctx context.Context, deviceID string, updates *DeviceUpdate) (*v1.Device, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is empty")
	}

	// Get current device
	device, err := uc.store.Devices().GetByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Apply updates
	if updates.Name != nil {
		device.Name = *updates.Name
	}
	if updates.Location != nil {
		device.Location = updates.Location
	}
	if updates.Metadata != nil {
		device.Metadata = updates.Metadata
	}
	if updates.Status != nil {
		device.Status = *updates.Status
	}

	// Update (not Save - Save is for creating new records)
	if err := uc.store.Devices().Update(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}

	return device, nil
}

// DeleteDevice removes a device
func (uc *DeviceUsecase) DeleteDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("device ID is empty")
	}

	// Delete associated data
	_ = uc.store.Actions().DeleteByDeviceID(ctx, deviceID)
	_ = uc.store.Messages().DeleteByDeviceID(ctx, deviceID)
	_ = uc.store.Certificates().DeleteByDevice(ctx, deviceID)

	// Delete device
	return uc.store.Devices().Delete(ctx, deviceID)
}

// DeviceUpdate represents device update fields
type DeviceUpdate struct {
	Name     *string
	Location *v1.DeviceLocation
	Metadata *v1.DeviceMetadata
	Status   *v1.DeviceStatus
}

// RegisterDeviceRequest for device registration workflow
type RegisterDeviceRequest struct {
	CreatedBy   string
	Name        string
	Location    *v1.DeviceLocation
	Company     *v1.CompanyDetailsExtended
	Certificate string // Optional: PEM-encoded X.509 certificate (device identification only)
}

// RegisterDeviceResponse for device registration workflow
type RegisterDeviceResponse struct {
	Device       *v1.DeviceSummary
	AuthToken    string
	Instructions map[string]string
}
