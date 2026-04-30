'use client';

import React, { useEffect, useState, useRef } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { MoreVertical, Settings, FileText, Trash2, AlertTriangle } from 'lucide-react';
import './Instances.css';

const Instances = () => {
    const router = useRouter();
    const [instances, setInstances] = useState([]);
    const [openDropdownId, setOpenDropdownId] = useState(null);
    const [expandedRows, setExpandedRows] = useState({});
    const [isLoading, setIsLoading] = useState(true);
    const [fetchError, setFetchError] = useState('');
    const [deleteModalConfig, setDeleteModalConfig] = useState({ isOpen: false, instance: null, isDeleting: false });
    const dropdownRef = useRef(null);

    const getErrorMessage = (data, fallback) => {
        return (
            data?.error?.message ||
            data?.message ||
            data?.details ||
            fallback
        );
    };

    useEffect(() => {
        const loadInstances = async () => {
            try {
                const storedData = localStorage.getItem('taksa_user');
                const parsedUser = storedData ? JSON.parse(storedData) : null;
                const createdBy = parsedUser?.email || '';

                if (!createdBy) {
                    setFetchError('User email not found. Please log in again.');
                    setIsLoading(false);
                    return;
                }

                const params = new URLSearchParams();
                params.set('created_by', createdBy);
                params.set('page_size', '20');

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

                setFetchError('');
                setInstances(Array.isArray(data?.devices) ? data.devices : []);
            } catch (error) {
                console.error('Failed to load Data Collecting Devices:', error);
                setFetchError(error.message || 'Failed to load Data Collecting Devices.');
            } finally {
                setIsLoading(false);
            }
        };

        loadInstances();

        const refreshInterval = setInterval(loadInstances, 180000);

        const handleClickOutside = (event) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
                setOpenDropdownId(null);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => {
            clearInterval(refreshInterval);
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, []);

    const toggleDropdown = (id) => {
        setOpenDropdownId(openDropdownId === id ? null : id);
    };

    const initiateDelete = (instance) => {
        setDeleteModalConfig({ isOpen: true, instance, isDeleting: false });
        setOpenDropdownId(null);
    };

    const cancelDelete = () => {
        if (deleteModalConfig.isDeleting) return;
        setDeleteModalConfig({ isOpen: false, instance: null, isDeleting: false });
    };

    const confirmDelete = async () => {
        const instanceId = deleteModalConfig.instance?.id;
        if (!instanceId) {
            setDeleteModalConfig({ isOpen: false, instance: null, isDeleting: false });
            return;
        }

        try {
            setDeleteModalConfig((prev) => ({ ...prev, isDeleting: true }));
            setFetchError('');

            const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(instanceId)}`, {
                method: 'DELETE',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to delete Data Collecting Device.'));
            }

            const updatedList = instances.filter(inst => inst.id !== instanceId);
            setInstances(updatedList);
            setOpenDropdownId(null);
            setDeleteModalConfig({ isOpen: false, instance: null, isDeleting: false });
        } catch (error) {
            console.error('Failed to delete Data Collecting Device:', error);
            setFetchError(error.message || 'Failed to delete Data Collecting Device.');
            setOpenDropdownId(null);
            setDeleteModalConfig((prev) => ({ ...prev, isDeleting: false }));
        }
    };

    const handleDeviceDetails = () => {
        setOpenDropdownId(null);
    };

    const handleConfigFile = (instance) => {
        const query = new URLSearchParams({
            deviceId: String(instance?.id || ''),
            deviceName: String(instance?.name || '')
        });

        router.push(`/dashboard/Edge-devices/config?${query.toString()}`);
        setOpenDropdownId(null);
    };

    const toggleExpandedRow = (id) => {
        setExpandedRows((prev) => ({ ...prev, [id]: !prev[id] }));
    };

    const getStatusMeta = (status) => {
        const statusByCode = {
            0: { key: 'unspecified', label: 'Unspecified' },
            1: { key: 'pending', label: 'Pending' },
            2: { key: 'active', label: 'Active' },
            3: { key: 'inactive', label: 'Inactive' },
            4: { key: 'suspended', label: 'Suspended' },
            5: { key: 'decommissioned', label: 'Decommissioned' }
        };

        if (typeof status === 'number' && statusByCode[status]) {
            return statusByCode[status];
        }

        const normalized = String(status ?? '').replace(/"/g, '').trim().toLowerCase();
        const statusByText = {
            unspecified: statusByCode[0],
            pending: statusByCode[1],
            active: statusByCode[2],
            inactive: statusByCode[3],
            suspended: statusByCode[4],
            decommissioned: statusByCode[5]
        };

        return statusByText[normalized] || { key: 'unknown', label: 'Unknown' };
    };

    const formatToIST = (dateValue) => {
        if (!dateValue) return '--';
        const parsedDate = new Date(dateValue);
        if (Number.isNaN(parsedDate.getTime())) return '--';

        return new Intl.DateTimeFormat('en-IN', {
            timeZone: 'Asia/Kolkata',
            day: '2-digit',
            month: 'short',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: true
        }).format(parsedDate);
    };

    const getSortedLevels = (instance) => {
        const levels = instance?.location?.levels;
        if (!levels || typeof levels !== 'object') return [];

        return Object.entries(levels)
            .filter(([key]) => /^\d+$/.test(key))
            .sort((a, b) => Number(a[0]) - Number(b[0]));
    };

    return (
        <div className="instances-container">
            <div className="instances-header-container">
                <div>
                    <h1 className="instances-title">Data Collecting Devices</h1>
                    <p className="instances-subtitle">
                        Set up, manage, and oversee all your data collecting devices. Adding a new device is the first step in establishing your Unified Namespace.
                    </p>
                </div>
                {instances.length > 0 && (
                    <Link href="/dashboard/Edge-devices/add" style={{ textDecoration: 'none' }}>
                        <button className="btn-black header-add-btn">Add DCD</button>
                    </Link>
                )}
            </div>

            {fetchError && (
                <div style={{ color: 'red', marginBottom: '16px', padding: '10px', background: '#ffeeee', borderRadius: '4px' }}>
                    {fetchError}
                </div>
            )}

            {isLoading ? (
                <div className="instances-empty-state">
                    <h3 className="empty-state-bold">Loading Data Collecting Devices...</h3>
                </div>
            ) : instances.length === 0 ? (
                <div className="instances-empty-state">
                    <h3 className="empty-state-bold">No Data Collecting Devices Available</h3>
                    <p className="empty-state-sub">Welcome! Let's get started by setting up your first data collecting device.</p>
                    <Link href="/dashboard/Edge-devices/add" style={{ textDecoration: 'none' }}>
                        <button className="btn-black">Add Your First DCD</button>
                    </Link>
                </div>
            ) : (
                <div className="instances-table-wrapper">
                    <table className="instances-table">
                        <thead>
                            <tr>
                                <th className="expand-col"></th>
                                <th>DCD Name</th>
                                <th>Status</th>
                                <th>Last Seen (IST)</th>
                                <th>Created At (IST)</th>
                                <th className="action-col">Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {instances.map((instance) => {
                                const isExpanded = !!expandedRows[instance.id];
                                const status = getStatusMeta(instance.status);
                                const levels = getSortedLevels(instance);

                                return (
                                    <React.Fragment key={instance.id}>
                                        <tr>
                                            <td className="expand-col">
                                                <button
                                                    className="expand-btn"
                                                    type="button"
                                                    onClick={() => toggleExpandedRow(instance.id)}
                                                    aria-label={isExpanded ? 'Collapse row' : 'Expand row'}
                                                >
                                                    {isExpanded ? '▾' : '▴'}
                                                </button>
                                            </td>
                                            <td><span className="device-name-text">{instance.name || '--'}</span></td>
                                            <td>
                                                <span className={`status-pill status-${status.key}`}>
                                                    {status.label}
                                                </span>
                                            </td>
                                            <td>{formatToIST(instance.lastSeen)}</td>
                                            <td>{formatToIST(instance.createdAt)}</td>
                                            <td className="action-col">
                                                <div className="action-cell">
                                                    <button className="action-btn" onClick={() => toggleDropdown(instance.id)}>
                                                        <MoreVertical size={20} />
                                                    </button>

                                                    {openDropdownId === instance.id && (
                                                        <div className="action-dropdown" ref={dropdownRef}>
                                                            <button type="button" className="dropdown-item" onClick={handleDeviceDetails}>
                                                                <Settings size={18} className="dropdown-item-icon" aria-hidden="true" />
                                                                <span>Device details</span>
                                                            </button>
                                                            <button type="button" className="dropdown-item" onClick={() => handleConfigFile(instance)}>
                                                                <FileText size={18} className="dropdown-item-icon" aria-hidden="true" />
                                                                <span>Config file</span>
                                                            </button>
                                                            <button type="button" className="dropdown-item text-danger" onClick={() => initiateDelete(instance)}>
                                                                <Trash2 size={18} className="dropdown-item-icon" aria-hidden="true" />
                                                                <span>Delete</span>
                                                            </button>
                                                        </div>
                                                    )}
                                                </div>
                                            </td>
                                        </tr>

                                        {isExpanded && (
                                            <tr className="levels-row">
                                                <td></td>
                                                <td colSpan={5}>
                                                    <div className="levels-content">
                                                        <div className="expanded-id-row">
                                                            <span className="level-label">ID:</span>
                                                            <span className="expanded-id-value">{instance.id || '--'}</span>
                                                        </div>
                                                        {levels.length > 0 ? (
                                                            levels.map(([level, value]) => (
                                                                <div className="level-item" key={`${instance.id}-${level}`}>
                                                                    <span className="level-label">Level {level}:</span>
                                                                    <span>{value}</span>
                                                                </div>
                                                            ))
                                                        ) : (
                                                            <div className="level-item">
                                                                <span className="level-label">Location:</span>
                                                                <span>Not available</span>
                                                            </div>
                                                        )}
                                                    </div>
                                                </td>
                                            </tr>
                                        )}
                                    </React.Fragment>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}

            {deleteModalConfig.isOpen && (
                <div className="instances-modal-overlay">
                    <div className="instances-delete-modal">
                        <div className="instances-delete-icon-wrapper">
                            <div className="instances-delete-icon-bg">
                                <AlertTriangle size={28} color="#dc2626" />
                            </div>
                        </div>
                        <h3 className="instances-delete-title">Delete Device?</h3>
                        <p className="instances-delete-desc">
                            Are you sure you want to delete device <strong>{deleteModalConfig.instance?.name || '--'}</strong>? This action cannot be undone and will be permanently removed.
                        </p>

                        <div className="instances-delete-actions">
                            <button
                                className="instances-btn-cancel"
                                onClick={cancelDelete}
                                disabled={deleteModalConfig.isDeleting}
                            >
                                Cancel
                            </button>
                            <button
                                className="instances-btn-danger"
                                onClick={confirmDelete}
                                disabled={deleteModalConfig.isDeleting}
                            >
                                {deleteModalConfig.isDeleting ? 'Deleting...' : 'Yes, Delete'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Instances;