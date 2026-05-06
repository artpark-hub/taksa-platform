package biz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/pkg/cert"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
	"github.com/artpark-hub/taksa-platform/device-management/internal/utils"
)

// DeviceUsecase handles device business logic (registration, listing, updates)
type DeviceUsecase struct {
	store              storage.Store
	authUc             *AuthUsecase
	baseURL            string
	umhCoreDockerImage string
}

const deviceActiveWindow = 1 * time.Minute

// NewDeviceUsecase creates a new device use case
func NewDeviceUsecase(store storage.Store, authUc *AuthUsecase, baseURL, umhCoreDockerImage string) *DeviceUsecase {
	return &DeviceUsecase{
		store:              store,
		authUc:             authUc,
		baseURL:            baseURL,
		umhCoreDockerImage: umhCoreDockerImage,
	}
}

// RegisterDevice creates a new device and auth token
func (uc *DeviceUsecase) RegisterDevice(ctx context.Context, req *RegisterDeviceRequest) (*RegisterDeviceResponse, error) {
	// Validate
	if req.CreatedBy == "" || req.Name == "" || req.Location == nil {
		return nil, fmt.Errorf("createdBy, name, and location are required")
	}

	// Check if already exists by createdBy + name (per-tenant unique)
	existing, _ := uc.store.Devices().GetByCreatedByAndName(ctx, req.CreatedBy, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("device with name '%s' already exists for this tenant", req.Name)
	}

	device := &v1.Device{
		Id:        generateUUID(),
		CreatedBy: req.CreatedBy,
		Name:      req.Name,
		Location:  req.Location,
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

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id not found in context")
	}

	// Save device FIRST (before certificate/token creation so FK constraints are satisfied)
	if err := uc.store.Devices().Save(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to save device: %w", err)
	}

	// Save device certificate AFTER device exists with tenant isolation
	if err := uc.store.Certificates().DeviceStore().SaveDevice(ctx, tenantID, device.Id, deviceCert); err != nil {
		return nil, fmt.Errorf("failed to create device certificate: %w", err)
	}

	// Create auth token AFTER device is saved with tenant isolation
	token, err := uc.authUc.CreateAuthToken(ctx, tenantID, device.Id, 7)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth token: %w", err)
	}

	// Set auth token expiry on device (50 years from now - matches token creation logic)
	tokenExpiryTime := time.Now().AddDate(50, 0, 0)
	device.AuthTokenExpiresAt = timestamppb.New(tokenExpiryTime)

	// Update device with token expiry timestamp
	if err := uc.store.Devices().Update(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to update device with token expiry: %w", err)
	}

	// Generate Docker command for device deployment
	dockerCmd := uc.buildDockerCommand(device.Name, device.Location, token)

	return &RegisterDeviceResponse{
		Device:    deviceToSummary(device),
		AuthToken: token,
		Instructions: map[string]string{
			"docker_command": dockerCmd,
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
	effectiveStatus := utils.DeriveEffectiveDeviceStatus(device, deviceActiveWindow)

	// Get recent messages (avoid ListHistory COUNT query; health only needs latest N)
	messages, listErr := uc.store.Messages().GetRecentByDevice(ctx, deviceID, 25)

	// Build response based on device's current state
	resp := &GetDeviceHealthResponse{
		DeviceId:    deviceID,
		DeviceStatus: effectiveStatus,
		LastSeen:    device.LastSeen, // Use device's last_seen from Pull API heartbeat
		// /health is intentionally summary-only; we do not surface full StatusMessage payloads here.
	}
	if listErr != nil {
		resp.ErrorMessage = "failed to load device message history for status heartbeat"
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

	// If device has sent messages, extract summary health data from the most recent status heartbeat.
	// We cannot assume the newest message is a status heartbeat (action-replies may arrive more frequently).
	foundStatusHeartbeat := false
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if msg.Content == "" {
			continue
		}
		// taksa-edge-umh sends MessageType "status" (not "status-message")
		if msg.Type != "StatusMessage" && msg.Type != "status-message" && msg.Type != "status" {
			continue
		}
		foundStatusHeartbeat = true
		core, latency, resources, release, deviceTs := extractDeviceHealthSummariesFromStatusContent(msg.Content)
		resp.CoreHealth = core
		resp.AgentLatency = latency
		resp.Resources = resources
		resp.Release = release
		resp.DeviceTimestamp = deviceTs
		break
	}
	if len(messages) > 0 && !foundStatusHeartbeat {
		// Most likely: device isn't pushing StatusMessage heartbeats into DM (or message_type differs).
		// Keep endpoint summary-only but make the gap visible to clients/operators.
		if resp.ErrorMessage == "" {
			resp.ErrorMessage = "no status heartbeat message found in recent device messages (expected message_type: status/status-message/StatusMessage)"
		} else {
			resp.ErrorMessage = resp.ErrorMessage + "; no status heartbeat message found in recent device messages (expected message_type: status/status-message/StatusMessage)"
		}
	}

	return resp, nil
}

func extractDeviceHealthSummariesFromStatusContent(content string) (*v1.DeviceCoreHealthSummary, *v1.DeviceAgentLatencySummary, *v1.DeviceResourcesSummary, *v1.DeviceReleaseSummary, *timestamppb.Timestamp) {
	decoded := content
	if b, err := base64.StdEncoding.DecodeString(content); err == nil && len(b) > 0 {
		decoded = string(b)
	}
	if decompressed, err := decompressIfNeeded([]byte(decoded)); err == nil && len(decompressed) > 0 {
		decoded = string(decompressed)
	}

	var root map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(decoded))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil || root == nil {
		return nil, nil, nil, nil, nil
	}

	// The status heartbeat may be either the StatusMessage object itself or a wrapper with "Payload".
	payload := root
	if p, ok := root["Payload"].(map[string]interface{}); ok && p != nil {
		payload = p
	}

	core := jsonMap(payload, "core", "Core")
	if core == nil {
		return nil, nil, nil, nil, nil
	}

	coreHealth := jsonMap(core, "health", "Health")
	outCoreHealth := &v1.DeviceCoreHealthSummary{}
	if coreHealth != nil {
		outCoreHealth.Message = jsonString(coreHealth, "message", "Message")
		outCoreHealth.State = jsonString(coreHealth, "state", "State")
		outCoreHealth.DesiredState = jsonString(coreHealth, "desiredState", "desired_state", "DesiredState")
		outCoreHealth.Category = jsonString(coreHealth, "category", "Category")
		if outCoreHealth.Message == "" && outCoreHealth.State == "" && outCoreHealth.DesiredState == "" && outCoreHealth.Category == "" {
			outCoreHealth = nil
		}
	} else {
		outCoreHealth = nil
	}

	agent := jsonMap(core, "agent", "Agent")
	outLatency := (*v1.DeviceAgentLatencySummary)(nil)
	if agent != nil {
		lat := jsonMap(agent, "latency", "Latency")
		if lat != nil {
			outLatency = &v1.DeviceAgentLatencySummary{
				AvgMs: jsonFloat(lat, "avgMs", "avg_ms", "AvgMs"),
				MaxMs: jsonFloat(lat, "maxMs", "max_ms", "MaxMs"),
				MinMs: jsonFloat(lat, "minMs", "min_ms", "MinMs"),
				P95Ms: jsonFloat(lat, "p95Ms", "p95_ms", "P95Ms"),
				P99Ms: jsonFloat(lat, "p99Ms", "p99_ms", "P99Ms"),
			}
			if outLatency.AvgMs == 0 && outLatency.MaxMs == 0 && outLatency.MinMs == 0 && outLatency.P95Ms == 0 && outLatency.P99Ms == 0 {
				outLatency = nil
			}
		}
	}

	container := jsonMap(core, "container", "Container")
	outResources := (*v1.DeviceResourcesSummary)(nil)
	if container != nil {
		outResources = &v1.DeviceResourcesSummary{
			Hwid:         jsonString(container, "hwid", "Hwid"),
			Architecture: jsonString(container, "architecture", "Architecture"),
		}
		if cpu := jsonMap(container, "cpu", "CPU"); cpu != nil {
			outResources.CpuTotalUsageMCpu = jsonFloat(cpu, "totalUsageMCpu", "cpu_total_usage_m_cpu", "TotalUsageMCpu")
			outResources.CpuCoreCount = int32(jsonFloat(cpu, "coreCount", "cpu_core_count", "CoreCount"))
			outResources.CpuCgroupCores = jsonFloat(cpu, "cgroupCores", "cpu_cgroup_cores", "CgroupCores")
			outResources.CpuThrottleRatio = jsonFloat(cpu, "throttleRatio", "cpu_throttle_ratio", "ThrottleRatio")
			outResources.CpuIsThrottled = jsonBool(cpu, "isThrottled", "cpu_is_throttled", "IsThrottled")
		}
		if mem := jsonMap(container, "memory", "Memory"); mem != nil {
			outResources.MemoryUsedBytes = jsonInt64(mem, "cGroupUsedBytes", "cgroupUsedBytes", "memory_used_bytes", "CGroupUsedBytes")
			outResources.MemoryTotalBytes = jsonInt64(mem, "cGroupTotalBytes", "cgroupTotalBytes", "memory_total_bytes", "CGroupTotalBytes")
		}
		if disk := jsonMap(container, "disk", "Disk"); disk != nil {
			outResources.DiskUsedBytes = jsonInt64(disk, "dataPartitionUsedBytes", "disk_used_bytes", "DataPartitionUsedBytes")
			outResources.DiskTotalBytes = jsonInt64(disk, "dataPartitionTotalBytes", "disk_total_bytes", "DataPartitionTotalBytes")
		}
		if outResources.Hwid == "" &&
			outResources.Architecture == "" &&
			outResources.CpuTotalUsageMCpu == 0 &&
			outResources.CpuCoreCount == 0 &&
			outResources.CpuCgroupCores == 0 &&
			outResources.CpuThrottleRatio == 0 &&
			!outResources.CpuIsThrottled &&
			outResources.MemoryUsedBytes == 0 &&
			outResources.MemoryTotalBytes == 0 &&
			outResources.DiskUsedBytes == 0 &&
			outResources.DiskTotalBytes == 0 {
			outResources = nil
		}
	}

	release := jsonMap(core, "release", "Release")
	outRelease := (*v1.DeviceReleaseSummary)(nil)
	if release != nil {
		outRelease = &v1.DeviceReleaseSummary{
			Version: jsonString(release, "version", "Version"),
			Channel: jsonString(release, "channel", "Channel"),
		}
		if outRelease.Version == "" && outRelease.Channel == "" {
			outRelease = nil
		}
	}

	// DeviceTimestamp: status payload does not currently include an explicit timestamp field in v2.StatusMessage.
	return outCoreHealth, outLatency, outResources, outRelease, nil
}

func jsonMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k].(map[string]interface{}); ok {
			return v
		}
	}
	return nil
}

func jsonString(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			return s
		}
	}
	return ""
}

func jsonFloat(m map[string]interface{}, keys ...string) float64 {
	if m == nil {
		return 0
	}
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		case json.Number:
			if f, err := v.Float64(); err == nil {
				return f
			}
		}
	}
	return 0
}

func jsonInt64(m map[string]interface{}, keys ...string) int64 {
	if m == nil {
		return 0
	}
	for _, k := range keys {
		switch v := m[k].(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case int32:
			return int64(v)
		case float64:
			// Best-effort fallback (should be avoided for large integers)
			return int64(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return i
			}
			if f, err := v.Float64(); err == nil {
				return int64(f)
			}
		case string:
			if i, err := json.Number(v).Int64(); err == nil {
				return i
			}
		}
	}
	return 0
}

func jsonBool(m map[string]interface{}, keys ...string) bool {
	if m == nil {
		return false
	}
	for _, k := range keys {
		if b, ok := m[k].(bool); ok {
			return b
		}
	}
	return false
}

// GetDeviceHealthResponse contains health metrics
type GetDeviceHealthResponse struct {
	DeviceId        string
	DeviceStatus    v1.DeviceStatus
	LastSeen        *timestamppb.Timestamp
	DeviceTimestamp *timestamppb.Timestamp
	ErrorMessage    string
	CoreHealth      *v1.DeviceCoreHealthSummary
	AgentLatency    *v1.DeviceAgentLatencySummary
	Resources       *v1.DeviceResourcesSummary
	Release         *v1.DeviceReleaseSummary
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

	// Multi-tenancy: extract tenant_id from context for deletion queries
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant_id not found in context")
	}

	// Delete associated data with tenant isolation
	_ = uc.store.Actions().DeleteByDeviceID(ctx, tenantID, deviceID)
	_ = uc.store.AuthTokens().DeleteByDeviceID(ctx, tenantID, deviceID)
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
	Certificate string // Optional: PEM-encoded X.509 certificate (device identification only)
}

// RegisterDeviceResponse for device registration workflow
type RegisterDeviceResponse struct {
	Device       *v1.DeviceSummary
	AuthToken    string
	Instructions map[string]string
}

// buildDockerCommand generates a ready-to-use Docker command for umh-core deployment
func (uc *DeviceUsecase) buildDockerCommand(
	deviceName string,
	location *v1.DeviceLocation,
	authToken string,
) string {
	// Set defaults if config values are missing
	baseURL := uc.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	dockerImage := uc.umhCoreDockerImage
	if dockerImage == "" {
		dockerImage = "management.umh.app/oci/united-manufacturing-hub/umh-core:v0.43.17"
	}

	// Sanitize device name for container and volume naming
	sanitizedName := sanitizeDeviceName(deviceName)

	// Build base command
	cmd := fmt.Sprintf("docker run -d --restart unless-stopped --name umh-core-%s", sanitizedName)

	// Add named volume mount (docker volume instead of bind mount)
	cmd += fmt.Sprintf(" -v umh-core-data-%s:/data", sanitizedName)

	// Add auth token
	cmd += fmt.Sprintf(" -e AUTH_TOKEN=%s", authToken)

	// Add release channel
	cmd += " -e RELEASE_CHANNEL=stable"

	// Add LOCATION_* environment variables from the 7-level hierarchy
	// Keys are "0" through "6" per ISA-95 levels
	if location != nil && location.Levels != nil {
		for i := 0; i < 7; i++ {
			levelKey := fmt.Sprintf("%d", i)
			if val, exists := location.Levels[levelKey]; exists && val != "" {
				cmd += fmt.Sprintf(" -e LOCATION_%d=%s", i, val)
			}
		}
	}

	// Add API URL
	apiURL := fmt.Sprintf("%s/api", strings.TrimSuffix(baseURL, "/"))
	cmd += fmt.Sprintf(" -e API_URL=%s", apiURL)

	// Add docker image
	cmd += fmt.Sprintf(" %s", dockerImage)

	return cmd
}

// sanitizeDeviceName removes special characters for use in container/dir names
func sanitizeDeviceName(name string) string {
	// Replace non-alphanumeric characters with hyphens
	reg := regexp.MustCompile("[^a-zA-Z0-9-]")
	sanitized := reg.ReplaceAllString(name, "-")
	// Remove consecutive hyphens
	reg = regexp.MustCompile("-+")
	return reg.ReplaceAllString(sanitized, "-")
}
