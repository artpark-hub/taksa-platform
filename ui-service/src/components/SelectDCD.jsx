'use client';

import React, { useEffect, useState } from 'react';
import { ChevronRight } from 'lucide-react';
import { useRouter } from 'next/navigation';
import './SelectDCD.css';

const SelectDCD = () => {
    const router = useRouter();
    const [devices, setDevices] = useState([]);
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

    const getDeviceId = (device) => {
        return String(device?.id || device?.uuid || '');
    };

    const getDeviceName = (device) => {
        return device?.name || device?.displayName || device?.display_name || 'Unnamed DCD';
    };

    const getDeviceStatus = (device) => {
        const status = device?.status;

        if (typeof status === 'number') {
            const statusByCode = {
                0: 'Unspecified',
                1: 'Pending',
                2: 'Active',
                3: 'Inactive',
                4: 'Suspended',
                5: 'Decommissioned'
            };

            return statusByCode[status] || 'Unknown';
        }

        return String(status || 'Unknown');
    };

    const handleDeviceSelect = (device) => {
        const query = new URLSearchParams({
            deviceId: getDeviceId(device),
            deviceName: getDeviceName(device)
        });

        router.push(`/dashboard/bridges/list?${query.toString()}`);
    };

    useEffect(() => {
        let cancelled = false;

        const loadDevices = async () => {
            try {
                setIsLoading(true);

                const params = new URLSearchParams();
                params.set('status', 'ACTIVE');

                const response = await fetch(`/api/v1/devicemgmt/devices?${params.toString()}`, {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                });

                const data = await response.json().catch(() => ({}));

                if (!response.ok) {
                    throw new Error(getErrorMessage(data, 'Failed to load Data Collecting Devices.'));
                }

                if (!cancelled) {
                    setFetchError('');
                    setDevices(Array.isArray(data?.devices) ? data.devices : []);
                }
            } catch (error) {
                if (!cancelled) {
                    console.error('Failed to load Data Collecting Devices:', error);
                    setFetchError(error.message || 'Failed to load Data Collecting Devices.');
                    setDevices([]);
                }
            } finally {
                if (!cancelled) {
                    setIsLoading(false);
                }
            }
        };

        loadDevices();

        return () => {
            cancelled = true;
        };
    }, []);

    return (
        <div className="select-dcd-container">
            <div className="select-dcd-header">
                <h1 className="select-dcd-title">Select DCD</h1>
                <p className="select-dcd-subtitle">
                    Select the DCD whose bridges you want to see or configure.
                </p>
            </div>

            {fetchError && <div className="select-dcd-error-msg">{fetchError}</div>}

            {isLoading ? (
                <div className="select-dcd-empty-state">
                    <p className="select-dcd-empty-loading">Loading DCDs...</p>
                </div>
            ) : devices.length === 0 ? (
                <div className="select-dcd-empty-state">
                    <h3 className="select-dcd-empty-bold">No DCD Found</h3>
                    <p className="select-dcd-empty-sub">
                        No active devices were found for your account.
                    </p>
                </div>
            ) : (
                <div className="select-dcd-grid">
                    {devices.map((device) => (
                        <button
                            key={getDeviceId(device)}
                            className="select-dcd-card"
                            onClick={() => handleDeviceSelect(device)}
                        >
                            <div className="select-dcd-card-content">
                                <div className="select-dcd-card-top">
                                    <h3>{getDeviceName(device)}</h3>
                                    <span className={`select-dcd-status-pill${getDeviceStatus(device).toLowerCase() === 'active' ? ' select-dcd-status-active' : ''}`}>{getDeviceStatus(device)}</span>
                                </div>
                                <p className="select-dcd-card-id">ID: {getDeviceId(device) || '--'}</p>
                            </div>

                            <ChevronRight size={22} className="select-dcd-chevron" />
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
};

export default SelectDCD;
