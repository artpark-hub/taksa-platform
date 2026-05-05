'use client';

import React, { useEffect, useState } from 'react';
import { ArrowLeft } from 'lucide-react';
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

        router.push(`/bridges/add${query.toString() ? `?${query.toString()}` : ''}`);
    };

    const handleBackToSelectDcd = () => {
        router.push('/bridges');
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
                            </tr>
                        </thead>
                        <tbody>
                            {bridges.map((bridge) => {
                                const deploymentStatus = getDeploymentStatusMeta(getDeploymentStatus(bridge));
                                const healthStatus = getHealthStatusMeta(getHealthStatus(bridge));

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
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
};

export default Bridges;