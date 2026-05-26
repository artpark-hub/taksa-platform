package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"
	"gopkg.in/yaml.v3"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

const (
	// natsMirrorDataFlowName is the stable umh-core data flow component name (idempotent deploy).
	natsMirrorDataFlowName = "UNS-to-NATS-mirror"
	// actionTypeDeployDataFlow must match umh-core models.DeployDataFlowComponent ("deploy-data-flow-component").
	actionTypeDeployDataFlow = "deploy-data-flow-component"
	// actionTypeEditDataFlow must match umh-core models.EditDataFlowComponent ("edit-data-flow-component").
	actionTypeEditDataFlow = "edit-data-flow-component"
	// natsMirrorPayloadTypeURL is stored with the queued action JSON; umh-core consumes raw JSON bytes.
	natsMirrorPayloadTypeURL = "type.googleapis.com/taksa.edge.NATSMirrorDeployPayload"
)

func parseNatsMirrorURLs(dep *conf.Deployment) []string {
	if dep == nil {
		return nil
	}
	raw := strings.TrimSpace(dep.GetNatsMirrorUrls())
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		u := strings.TrimSpace(p)
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}

// NATSMirrorConfigFingerprint returns a stable hash of the configured NATS URL list.
func NATSMirrorConfigFingerprint(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	sorted := append([]string(nil), urls...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(sum[:])
}

// natsMirrorComponentUUID matches umh-core dataflowcomponentserviceconfig.GenerateUUIDFromName.
func natsMirrorComponentUUID() string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(natsMirrorDataFlowName)).String()
}

func buildNATSMirrorCustomDFC(tenantID, deviceID string, natsURLs []string) (map[string]interface{}, error) {
	if len(natsURLs) == 0 {
		return nil, fmt.Errorf("no NATS URLs")
	}

	unsRoot := map[string]interface{}{
		"uns": map[string]interface{}{
			"consumer_group": "dm_uns_nats_mirror_" + strings.ReplaceAll(deviceID, "-", ""),
			"umh_topic":      ".*",
		},
	}
	unsYAML, err := yaml.Marshal(unsRoot)
	if err != nil {
		return nil, fmt.Errorf("marshal uns yaml: %w", err)
	}

	subject := fmt.Sprintf(
		`%s.%s.${! meta("umh_topic").re_replace("^umh\\.v1", "uns.v1") }`,
		tenantID,
		deviceID,
	)
	natsRoot := map[string]interface{}{
		"nats": map[string]interface{}{
			"urls":           natsURLs,
			"subject":        subject,
			"max_reconnects": -1,
		},
	}
	natsYAML, err := yaml.Marshal(natsRoot)
	if err != nil {
		return nil, fmt.Errorf("marshal nats yaml: %w", err)
	}

	procData := "mapping: |\n  root = this\n"

	return map[string]interface{}{
		"inputs": map[string]interface{}{
			"type": "uns",
			"data": string(unsYAML),
		},
		"outputs": map[string]interface{}{
			"type": "nats",
			"data": string(natsYAML),
		},
		"pipeline": map[string]interface{}{
			"processors": map[string]interface{}{
				"0": map[string]interface{}{
					"type": "bloblang",
					"data": procData,
				},
			},
		},
		"rawYAML": map[string]interface{}{
			"data": "buffer:\n  none: {}\n",
		},
	}, nil
}

// buildNATSMirrorDeployActionPayload builds the JSON body umh-core expects for deploy-data-flow-component.
func buildNATSMirrorDeployActionPayload(tenantID, deviceID string, natsURLs []string) (map[string]interface{}, error) {
	cdfc, err := buildNATSMirrorCustomDFC(tenantID, deviceID, natsURLs)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":              natsMirrorDataFlowName,
		"meta":              map[string]interface{}{"type": "custom"},
		"state":             "active",
		"ignoreHealthCheck": false,
		"payload": map[string]interface{}{
			"customDataFlowComponent": cdfc,
		},
	}, nil
}

// buildNATSMirrorEditActionPayload builds the JSON body umh-core expects for edit-data-flow-component.
func buildNATSMirrorEditActionPayload(tenantID, deviceID string, natsURLs []string) (map[string]interface{}, error) {
	cdfc, err := buildNATSMirrorCustomDFC(tenantID, deviceID, natsURLs)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"uuid":              natsMirrorComponentUUID(),
		"name":              natsMirrorDataFlowName,
		"meta":              map[string]interface{}{"type": "custom"},
		"state":             "active",
		"ignoreHealthCheck": false,
		"payload": map[string]interface{}{
			"customDataFlowComponent": cdfc,
		},
	}, nil
}

func (uc *InstanceUsecase) queueNATSMirrorAction(ctx context.Context, deviceID, actionType string, inner map[string]interface{}) {
	payloadJSON, err := json.Marshal(inner)
	if err != nil {
		fmt.Printf("Warning: marshal NATS mirror %s for device %s: %v\n", actionType, deviceID, err)
		return
	}
	_, err = uc.actionUc.QueueAction(ctx, &QueueActionRequest{
		DeviceID:   deviceID,
		ActionType: actionType,
		Payload: &anypb.Any{
			TypeUrl: natsMirrorPayloadTypeURL,
			Value:   payloadJSON,
		},
		MaxRetries: 3,
		TTLSeconds: 3600,
	})
	if err != nil {
		fmt.Printf("Warning: failed to queue NATS mirror %s for device %s: %v\n", actionType, deviceID, err)
	}
}

func (uc *InstanceUsecase) ensureNATSMirrorForDevice(ctx context.Context, deviceID string) {
	if uc == nil || uc.actionUc == nil || uc.store == nil {
		return
	}
	if deviceID == "" {
		return
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return
	}
	natsURLs := parseNatsMirrorURLs(uc.deployment)
	if len(natsURLs) == 0 {
		return
	}
	currentFP := NATSMirrorConfigFingerprint(natsURLs)

	deployed, err := uc.store.Devices().NATSMirrorDeployed(ctx, deviceID)
	if err != nil {
		fmt.Printf("Warning: NATSMirrorDeployed check failed for device %s: %v\n", deviceID, err)
		return
	}

	if deployed {
		storedFP, err := uc.store.Devices().GetNATSMirrorConfigFingerprint(ctx, deviceID)
		if err != nil {
			fmt.Printf("Warning: GetNATSMirrorConfigFingerprint for device %s: %v\n", deviceID, err)
			return
		}
		if storedFP == currentFP {
			return
		}
	}

	inflight, err := uc.store.Actions().NATSMirrorActionInflight(ctx, tenantID, deviceID)
	if err != nil {
		fmt.Printf("Warning: NATSMirrorActionInflight failed for device %s: %v\n", deviceID, err)
		return
	}
	if inflight {
		return
	}

	if !deployed {
		inner, err := buildNATSMirrorDeployActionPayload(tenantID, deviceID, natsURLs)
		if err != nil {
			fmt.Printf("Warning: build NATS mirror deploy payload for device %s: %v\n", deviceID, err)
			return
		}
		uc.queueNATSMirrorAction(ctx, deviceID, actionTypeDeployDataFlow, inner)
		return
	}

	inner, err := buildNATSMirrorEditActionPayload(tenantID, deviceID, natsURLs)
	if err != nil {
		fmt.Printf("Warning: build NATS mirror edit payload for device %s: %v\n", deviceID, err)
		return
	}
	uc.queueNATSMirrorAction(ctx, deviceID, actionTypeEditDataFlow, inner)
}

// EnsureUNSToNATSMirror queues deploy or edit so UNS is mirrored to NATS with subject pattern
// <tenant_id>.<device_id>.uns.v1.<rest> (only the leading umh.v1 segment is rewritten).
func (uc *InstanceUsecase) EnsureUNSToNATSMirror(ctx context.Context, deviceID string) {
	uc.ensureNATSMirrorForDevice(ctx, deviceID)
}

// ReconcileNATSMirrorFleet queues mirror deploy/edit for all devices whose stored config fingerprint
// differs from deployment.nats_mirror_urls (e.g. after .env or config.yaml change and DM restart).
func (uc *InstanceUsecase) ReconcileNATSMirrorFleet(ctx context.Context) {
	if uc == nil || uc.store == nil {
		return
	}
	natsURLs := parseNatsMirrorURLs(uc.deployment)
	if len(natsURLs) == 0 {
		return
	}
	currentFP := NATSMirrorConfigFingerprint(natsURLs)
	refs, err := uc.store.Devices().ListNATSMirrorDevicesNeedingUpdate(ctx, currentFP)
	if err != nil {
		fmt.Printf("Warning: ListNATSMirrorDevicesNeedingUpdate: %v\n", err)
		return
	}
	for _, ref := range refs {
		deviceCtx := middleware.SetTenantID(ctx, ref.TenantID)
		uc.ensureNATSMirrorForDevice(deviceCtx, ref.ID)
	}
}

// StartNATSMirrorFleetReconcile runs ReconcileNATSMirrorFleet once after a short delay (DB ready).
func (uc *InstanceUsecase) StartNATSMirrorFleetReconcile() {
	go func() {
		time.Sleep(3 * time.Second)
		uc.ReconcileNATSMirrorFleet(context.Background())
	}()
}

// IsNATSMirrorDeployAction reports whether the queued action deploys the platform UNS→NATS mirror DFC.
func IsNATSMirrorDeployAction(action *models.Action) bool {
	if action == nil || action.Type != actionTypeDeployDataFlow {
		return false
	}
	return natsMirrorActionPayloadMatches(action)
}

// IsNATSMirrorEditAction reports whether the queued action edits the platform UNS→NATS mirror DFC.
func IsNATSMirrorEditAction(action *models.Action) bool {
	if action == nil || action.Type != actionTypeEditDataFlow {
		return false
	}
	return natsMirrorActionPayloadMatches(action)
}

// IsNATSMirrorMirrorAction reports deploy or edit of UNS-to-NATS-mirror.
func IsNATSMirrorMirrorAction(action *models.Action) bool {
	return IsNATSMirrorDeployAction(action) || IsNATSMirrorEditAction(action)
}

func natsMirrorActionPayloadMatches(action *models.Action) bool {
	if action.Payload == nil || len(action.Payload.Value) == 0 {
		return false
	}
	return strings.Contains(string(action.Payload.Value), natsMirrorDataFlowName)
}

// RecordNATSMirrorDeploySuccess persists mirror success on the device row (survives actions cleanup).
func (uc *InstanceUsecase) RecordNATSMirrorDeploySuccess(ctx context.Context, action *models.Action) {
	uc.recordNATSMirrorApplySuccess(ctx, action)
}

func (uc *InstanceUsecase) recordNATSMirrorApplySuccess(ctx context.Context, action *models.Action) {
	if uc == nil || uc.store == nil || action == nil || !IsNATSMirrorMirrorAction(action) {
		return
	}
	fp := NATSMirrorConfigFingerprint(parseNatsMirrorURLs(uc.deployment))
	if err := uc.store.Devices().SetNATSMirrorApplied(ctx, action.DeviceId, time.Now(), fp); err != nil {
		fmt.Printf("Warning: SetNATSMirrorApplied for device %s: %v\n", action.DeviceId, err)
	}
}

func isNATSMirrorEditNotFoundError(errorMessage string) bool {
	return strings.Contains(strings.ToLower(errorMessage), "not found")
}

// HandleNATSMirrorActionFailure clears mirror state when an edit fails because the DFC is gone on the edge,
// then queues deploy (if NATS URLs are configured and no mirror action is inflight).
func (uc *InstanceUsecase) HandleNATSMirrorActionFailure(ctx context.Context, action *models.Action, finalStatus models.ActionStatus, errorMessage string) {
	if uc == nil || action == nil || !IsNATSMirrorEditAction(action) {
		return
	}
	if finalStatus != models.ActionStatusFailed {
		return
	}
	if !isNATSMirrorEditNotFoundError(errorMessage) {
		return
	}
	if err := uc.store.Devices().ClearNATSMirrorApplied(ctx, action.DeviceId); err != nil {
		fmt.Printf("Warning: ClearNATSMirrorApplied for device %s after edit not found: %v\n", action.DeviceId, err)
		return
	}
	fmt.Printf("Info: NATS mirror edit not found on device %s; cleared mirror state and re-queuing deploy\n", action.DeviceId)
	uc.ensureNATSMirrorForDevice(ctx, action.DeviceId)
}
