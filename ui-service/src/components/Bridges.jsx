'use client';

import React, { useEffect, useState } from 'react';
import { ArrowLeft, Settings, FileText, Grid3X3, Trash2, AlertTriangle } from 'lucide-react';
import { useRouter, useSearchParams } from 'next/navigation';
import './Bridges.css';

const Bridges = () => {
    const router = useRouter();
    const searchParams = useSearchParams();
    const deviceId = searchParams?.get('deviceId') || '';
    const deviceName = searchParams?.get('deviceName') || '';

    const [bridges, setBridges] = useState([]);
    const [isLoading, setIsLoading] = useState(true);
    const [fetchError, setFetchError] = useState('');
    const [openMenuBridgeId, setOpenMenuBridgeId] = useState(null);
    const [deleteBridgeId, setDeleteBridgeId] = useState(null);
    const [deleteBridgeName, setDeleteBridgeName] = useState('');
    const [isDeleting, setIsDeleting] = useState(false);
    const [deleteTimer, setDeleteTimer] = useState(0);
    const deleteCountdown = Math.max(45 - deleteTimer, 0);

    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

    const getErrorMessage = (data, fallback) => {
        return (
            data?.error?.message ||
            data?.message ||
            data?.details ||
            fallback
        );
    };

    const getBridgeId = (bridge) => {
        return bridge?.uuid || bridge?.id || '--';
    };

    const getBridgeName = (bridge) => {
        return bridge?.name || 'Unnamed Bridge';
    };

    const getBridgeType = (bridge) => {
        return bridge?.type || '--';
    };

    const getDeploymentStatus = (bridge) => {
        return bridge?.deploymentStatus || bridge?.deployment_status || '--';
    };

    const getDeploymentStatusMeta = (status) => {
        const normalized = String(status ?? '').replace(/"/g, '').trim().toLowerCase();

        const statusByText = {
            active: { key: 'active', label: 'Active' },
            inactive: { key: 'inactive', label: 'Inactive' },
            pending: { key: 'pending', label: 'Pending' },
            suspended: { key: 'suspended', label: 'Suspended' },
            decommissioned: { key: 'decommissioned', label: 'Decommissioned' },
            deploying: { key: 'pending', label: 'Deploying' },
            failed: { key: 'inactive', label: 'Failed' }
        };

        return statusByText[normalized] || { key: 'unknown', label: status || 'Unknown' };
    };

    const getHealthStatus = (bridge) => {
        return bridge?.healthStatus || bridge?.health_status || '--';
    };

    const getHealthStatusMeta = (status) => {
        const normalized = String(status ?? '').replace(/"/g, '').trim().toLowerCase();

        if (normalized === 'online') {
            return { key: 'online', label: 'Online' };
        }

        if (normalized === 'offline') {
            return { key: 'offline', label: 'Offline' };
        }

        return { key: 'unknown', label: status || 'Unknown' };
    };

    const getBridgeListFromResponse = (data) => {
        return (
            data?.converters ||
            data?.protocolConverters ||
            data?.protocol_converters ||
            data?.items ||
            data?.data?.converters ||
            data?.data?.protocolConverters ||
            data?.data?.protocol_converters ||
            []
        );
    };

    const handleAddBridge = () => {
        const query = new URLSearchParams();

        if (deviceId) {
            query.set('deviceId', deviceId);
        }

        if (deviceName) {
            query.set('deviceName', deviceName);
        }

        router.push(`/dashboard/bridges/add${query.toString() ? `?${query.toString()}` : ''}`);
    };

    const handleBackToSelectDcd = () => {
        router.push('/dashboard/bridges');
    };

    const fetchBridges = async () => {
        setIsLoading(true);

        try {
            if (!deviceId) {
                setFetchError('Device id is missing. Please open Bridges from a selected DCD.');
                setBridges([]);
                setIsLoading(false);
                return;
            }

            const storedData = localStorage.getItem('taksa_user');
            const parsedUser = storedData ? JSON.parse(storedData) : null;
            const createdBy = parsedUser?.email || '';

            if (!createdBy) {
                setFetchError('User email not found. Please log in again.');
                setBridges([]);
                setIsLoading(false);
                return;
            }

            const params = new URLSearchParams();
            params.set('created_by', createdBy);

            const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/protocol-converters?${params.toString()}`, {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to load bridge data flows.'));
            }

            setFetchError('');
            setBridges(Array.isArray(getBridgeListFromResponse(data)) ? getBridgeListFromResponse(data) : []);
        } catch (error) {
            console.error('Error fetching bridges:', error);
            setFetchError(error.message || 'Failed to load bridge data flows.');
            setBridges([]);
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        fetchBridges();
    }, [deviceId]);

    useEffect(() => {
        const handleClickOutside = (event) => {
            if (event.target.closest('.bridge-actions-cell')) {
                return;
            }

            setOpenMenuBridgeId(null);
        };

        document.addEventListener('mousedown', handleClickOutside);

        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, []);

    useEffect(() => {
        if (!isDeleting) {
            setDeleteTimer(0);
            return;
        }

        setDeleteTimer(0);
        const interval = setInterval(() => {
            setDeleteTimer((prev) => prev + 1);
        }, 1000);

        return () => clearInterval(interval);
    }, [isDeleting]);

    const pollProtocolConverterActionResult = async (actionId) => {
        for (let attempt = 0; attempt < 15; attempt += 1) {
            const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/protocol-converters/${encodeURIComponent(actionId)}/result`, {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to get bridge deletion status.'));
            }

            const statusText = String(data?.status ?? '').toUpperCase();
            const hasCompletedAt = Boolean(data?.completedAt);
            const hasError = Boolean(data?.errorMessage);
            const hasResult = Boolean(data?.result);

            if (hasError || statusText.includes('FAILED') || statusText === '3') {
                throw new Error(getErrorMessage(data, 'Bridge deletion failed.'));
            }

            if (statusText.includes('COMPLETED') || statusText === '2' || hasCompletedAt || hasResult) {
                return data;
            }

            await wait(3000);
        }

        throw new Error('Bridge deletion timed out after 45 seconds. Please try again.');
    };

    const deleteProtocolConverter = async (converterUuid) => {
        const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/protocol-converters/${encodeURIComponent(converterUuid)}`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to delete bridge.'));
        }

        const actionId = data?.actionId || data?.action_id || data?.id || data?.action?.id || '';

        if (!actionId) {
            throw new Error('Bridge deletion was queued but action id was not found.');
        }

        return actionId;
    };

    const handleDeleteConfirm = async () => {
        if (!deleteBridgeId || !deviceId) {
            setDeleteBridgeId(null);
            return;
        }

        try {
            setIsDeleting(true);
            const actionId = await deleteProtocolConverter(deleteBridgeId);
            await pollProtocolConverterActionResult(actionId);
            
            // Refresh the bridges list after successful deletion
            await fetchBridges();
            setDeleteBridgeId(null);
            setIsDeleting(false);
        } catch (error) {
            console.error('Error deleting bridge:', error);
            setFetchError(error.message || 'Failed to delete bridge.');
            setDeleteBridgeId(null);
            setIsDeleting(false);
        }
    };

    return (
        <div className="bridges-container">
            <div className="bridges-header-container">
                <div className="bridges-header-left">
                    <button className="bridges-back-btn" onClick={handleBackToSelectDcd}>
                        <ArrowLeft size={22} />
                    </button>

                    <div>
                        <h1 className="bridges-title">Bridge</h1>
                        <p className="bridges-subtitle">
                            Manage and customize your UMH Core bridge data flows powered by the open-source benthos-umh.
                        </p>
                    </div>
                </div>
                {bridges.length > 0 && (
                    <button className="bridges-btn-black bridges-header-add-btn" onClick={handleAddBridge}>
                        Add Bridge
                    </button>
                )}
            </div>

            {fetchError && (
                <div className="bridges-error-msg">
                    {fetchError}
                </div>
            )}

            {isLoading ? (
                <div className="bridges-empty-state">
                    <p className="bridges-empty-loading">Loading bridges...</p>
                </div>
            ) : bridges.length === 0 ? (
                <div className="bridges-empty-state">
                    <h3 className="bridges-empty-bold">No Bridge Data Flows Found</h3>
                    <p className="bridges-empty-sub">Get started by creating your first bridge data flow.</p>
                    <button className="bridges-btn-black" onClick={handleAddBridge}>
                        Add Your First Bridge
                    </button>
                </div>
            ) : (
                <div className="bridges-table-wrapper">
                    <table className="bridges-table">
                        <thead>
                            <tr>
                                <th>Name</th>
                                <th>Type</th>
                                <th>Deployment Status</th>
                                <th>Health</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {bridges.map((bridge) => {
                                const deploymentStatus = getDeploymentStatusMeta(getDeploymentStatus(bridge));
                                const healthStatus = getHealthStatusMeta(getHealthStatus(bridge));
                                const bridgeId = getBridgeId(bridge);
                                const showMenu = openMenuBridgeId === bridgeId;

                                const handleEditConfig = () => {
                                    const params = new URLSearchParams();
                                    
                                    params.set('bridgeId', bridgeId);
                                    if (deviceId) params.set('deviceId', deviceId);
                                    if (deviceName) params.set('deviceName', deviceName);
                                    
                                    setOpenMenuBridgeId(null);
                                    router.push(`/dashboard/bridges/edit?${params.toString()}`);
                                };

                                const handleLogs = () => {
                                    console.log('Logs:', bridgeId);
                                    setOpenMenuBridgeId(null);
                                };

                                const handleMetrics = () => {
                                    console.log('Metrics:', bridgeId);
                                    setOpenMenuBridgeId(null);
                                };

                                const handleDelete = () => {
                                    setDeleteBridgeId(bridgeId);
                                    setDeleteBridgeName(getBridgeName(bridge));
                                    setOpenMenuBridgeId(null);
                                };

                                return (
                                    <tr key={getBridgeId(bridge)}>
                                        <td>
                                            <span className="bridge-name-text">{getBridgeName(bridge)}</span>
                                        </td>
                                        <td>{getBridgeType(bridge)}</td>
                                        <td>
                                            <span className={`bridge-status-pill status-${deploymentStatus.key}`}>
                                                {deploymentStatus.label}
                                            </span>
                                        </td>
                                        <td>
                                            <span aria-label={`Health: ${healthStatus.label}`}>
                                                <span
                                                    className={`bridge-health-dot health-${healthStatus.key}`}
                                                    aria-hidden="true"
                                                ></span>
                                                {' '}
                                                <span className="bridge-health-label">{healthStatus.label}</span>
                                            </span>
                                        </td>
                                        <td>
                                            <div className="bridge-actions-cell">
                                                <button 
                                                    className="bridge-actions-menu-btn"
                                                    onClick={() => setOpenMenuBridgeId(openMenuBridgeId === bridgeId ? null : bridgeId)}
                                                    aria-label="Actions menu"
                                                >
                                                    ⋮
                                                </button>
                                                {showMenu && (
                                                    <div className="bridge-actions-menu">
                                                        <button 
                                                            className="bridge-action-item"
                                                            onClick={handleEditConfig}
                                                        >
                                                            <Settings size={18} />
                                                            Edit Config
                                                        </button>
                                                        <button 
                                                            className="bridge-action-item"
                                                            onClick={handleLogs}
                                                        >
                                                            <FileText size={18} />
                                                            Logs
                                                        </button>
                                                        <button 
                                                            className="bridge-action-item"
                                                            onClick={handleMetrics}
                                                        >
                                                            <Grid3X3 size={18} />
                                                            Metrics
                                                        </button>
                                                        <button 
                                                            className="bridge-action-item delete"
                                                            onClick={handleDelete}
                                                        >
                                                            <Trash2 size={18} />
                                                            Delete
                                                        </button>
                                                    </div>
                                                )}
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}

            {deleteBridgeId && !isDeleting && (
                <div className="bridge-delete-modal-overlay">
                    <div className="bridge-delete-modal">
                        <div className="bridge-delete-icon-wrapper">
                            <div className="bridge-delete-icon-bg">
                                <AlertTriangle size={28} color="#dc2626" />
                            </div>
                        </div>
                        <h3 className="bridge-delete-title">Delete Bridge?</h3>
                        <p className="bridge-delete-desc">
                            Are you sure you want to delete <strong>{deleteBridgeName}</strong>? This action will not be able to undone.
                        </p>
                        <div className="bridge-delete-modal-actions">
                            <button 
                                className="bridge-modal-cancel-btn"
                                onClick={() => setDeleteBridgeId(null)}
                            >
                                Cancel
                            </button>
                            <button 
                                className="bridge-modal-delete-btn"
                                onClick={handleDeleteConfirm}
                            >
                                Yes, Delete
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {isDeleting && (
                <div className="bridge-config-queue-overlay">
                    <div className="bridge-config-queue-modal">
                        <div className="bridge-config-loader"></div>
                        <h3>Deleting {deleteBridgeName}, kindly wait.</h3>
                        <p>The delete action has been queued and we are waiting for the device response.</p>
                        <p className="bridge-config-timer">Time left: {deleteCountdown}s</p>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Bridges;