'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { ArrowLeft, CloudUpload, FileText, Link, Network } from 'lucide-react';
import { useRouter, useSearchParams } from 'next/navigation';
import General from './General';
import Connection from './Connection';
import Readflow from './Readflow';
import './BridgeConfiguration.css';

const BridgeConfiguration = () => {
    const router = useRouter();
    const searchParams = useSearchParams();

    const initialProtocol = searchParams.get('protocol') || 'Modbus';
    const selectedDeviceId = searchParams.get('deviceId') || '';
    const selectedDeviceName = searchParams.get('deviceName') || '';

    const [activeTab, setActiveTab] = useState('general');
    const [isDeploying, setIsDeploying] = useState(false);
    const [deployMessage, setDeployMessage] = useState('');
    const [deployError, setDeployError] = useState('');

    const [bridgeConfig, setBridgeConfig] = useState({
        name: '',
        instance: selectedDeviceName,
        level0: '',
        levels: [],
        protocol: initialProtocol,
        dataType: 'Time Series',
        ipAddress: '',
        port: '',
        readInputType: 'modbus',
        readInputYaml: '',
        readProcessorType: 'tag_processor',
        readProcessorYaml: '',
        readRawYamlInject: 'buffer:\n  none: {}'
    });

    const getErrorMessage = (data, fallback) => {
        return (
            data?.error?.message ||
            data?.message ||
            data?.details ||
            fallback
        );
    };

    const getDeviceListFromResponse = (data) => {
        return Array.isArray(data?.devices)
            ? data.devices
            : Array.isArray(data?.items)
            ? data.items
            : [];
    };

    const getLevelsFromDevice = (device) => {
        const levels = device?.location?.levels;

        if (!levels || typeof levels !== 'object') {
            return [];
        }

        return Object.entries(levels)
            .filter(([key]) => /^\d+$/.test(String(key)))
            .sort((a, b) => Number(a[0]) - Number(b[0]))
            .map(([key, value]) => ({
                key: `level${key}`,
                index: Number(key),
                label: `Level ${key}`,
                value: String(value ?? '')
            }));
    };

    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

    const sanitizeProtocolMeta = (protocol) => {
        const normalized = String(protocol || '').trim().toLowerCase();

        if (normalized === 'modbus') {
            return 'modbus_tcp';
        }

        return normalized || 'modbus_tcp';
    };

    const toMultilineString = (value, fallback = '') => {
        const normalized = String(value ?? fallback);
        return normalized.endsWith('\n') ? normalized : `${normalized}\n`;
    };

    const getLocationPathValue = (location) => {
        const level0 = String(location?.['0'] || '').trim();
        return level0 || 'default-location';
    };

    const extractActionId = (data) => {
        return data?.actionId || data?.action_id || data?.id || data?.action?.id || '';
    };

    const extractConverterUuid = (data) => {
        return (
            data?.uuid ||
            data?.result?.uuid ||
            data?.result?.payload?.uuid ||
            data?.payload?.uuid ||
            data?.data?.uuid ||
            data?.component?.uuid ||
            ''
        );
    };

    const getDeployLocationFromBridgeConfig = () => {
        const levels = Array.isArray(bridgeConfig?.levels) ? bridgeConfig.levels : [];
        const normalizedLevels = levels
            .filter((level) => Number.isInteger(level?.index))
            .sort((a, b) => a.index - b.index);

        if (normalizedLevels.length > 0) {
            return normalizedLevels.reduce((acc, level) => {
                acc[String(level.index)] = String(level?.value || '');
                return acc;
            }, {});
        }

        return {
            '0': String(bridgeConfig?.level0 || '')
        };
    };

    const pollProtocolConverterActionResult = async (actionId) => {
        for (let attempt = 0; attempt < 60; attempt += 1) {
            const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/${encodeURIComponent(actionId)}/result`, {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to get bridge deployment status.'));
            }

            const statusText = String(data?.status ?? '').toUpperCase();
            const hasCompletedAt = Boolean(data?.completedAt);
            const hasError = Boolean(data?.errorMessage);
            const hasResult = Boolean(data?.result);

            if (hasError || statusText.includes('FAILED') || statusText === '3') {
                throw new Error(getErrorMessage(data, 'Bridge deployment failed.'));
            }

            if (statusText.includes('COMPLETED') || statusText === '2' || hasCompletedAt || hasResult) {
                return data;
            }

            await wait(3000);
        }

        throw new Error('Bridge deployment timed out. Please check bridge status and try again.');
    };

    const queueProtocolConverterUpdate = async (converterUuid, editPayload) => {
        const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/${encodeURIComponent(converterUuid)}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify({ payload: editPayload })
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to queue protocol converter edit request.'));
        }

        const actionId = extractActionId(data);

        if (!actionId) {
            throw new Error('Protocol converter edit was queued but action id was not found.');
        }

        return actionId;
    };

    useEffect(() => {
        let cancelled = false;

        const loadSelectedDevice = async () => {
            if (!selectedDeviceId) {
                setBridgeConfig((prev) => ({
                    ...prev,
                    instance: selectedDeviceName,
                    level0: '',
                    levels: []
                }));
                return;
            }

            try {
                const storedData = localStorage.getItem('taksa_user');
                const parsedUser = storedData ? JSON.parse(storedData) : null;
                const createdBy = parsedUser?.email || '';

                if (!createdBy) {
                    return;
                }

                const params = new URLSearchParams();
                params.set('created_by', createdBy);
                params.set('status', 'ACTIVE');
                params.set('page_size', '100');

                const response = await fetch(`/api/v1/devicemgmt/devices?${params.toString()}`, {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                });

                const data = await response.json().catch(() => ({}));

                if (!response.ok) {
                    throw new Error(getErrorMessage(data, 'Failed to load selected device details.'));
                }

                const devices = getDeviceListFromResponse(data);
                const selectedDevice = devices.find((device) =>
                    String(device?.id || device?.uuid || '') === String(selectedDeviceId)
                );

                const resolvedDeviceName =
                    selectedDevice?.name ||
                    selectedDevice?.displayName ||
                    selectedDevice?.display_name ||
                    selectedDeviceName;

                const resolvedLevels = getLevelsFromDevice(selectedDevice);

                if (!cancelled) {
                    setBridgeConfig((prev) => ({
                        ...prev,
                        instance: resolvedDeviceName,
                        level0: resolvedLevels[0]?.value || '',
                        levels: resolvedLevels
                    }));
                }
            } catch (error) {
                console.error('Failed to load selected device details:', error);
            }
        };

        loadSelectedDevice();

        return () => {
            cancelled = true;
        };
    }, [selectedDeviceId, selectedDeviceName]);

    const isStep3Ready = useMemo(() => {
        return Boolean(
            bridgeConfig.name &&
            bridgeConfig.instance &&
            bridgeConfig.level0 &&
            bridgeConfig.protocol &&
            bridgeConfig.dataType &&
            bridgeConfig.ipAddress &&
            bridgeConfig.port
        );
    }, [bridgeConfig]);

    const isOnReadflow = activeTab === 'readflow';
    const isStep3Active = isOnReadflow && isStep3Ready;
    const isSaveDeployEnabled = isStep3Active;

    const handleBack = () => {
        const query = new URLSearchParams();

        if (initialProtocol) {
            query.set('protocol', initialProtocol);
        }

        const deviceId = searchParams.get('deviceId');
        const deviceName = searchParams.get('deviceName');

        if (deviceId) {
            query.set('deviceId', deviceId);
        }

        if (deviceName) {
            query.set('deviceName', deviceName);
        }

        router.push(`/bridges/select-templates${query.toString() ? `?${query.toString()}` : ''}`);
    };

    const handleSaveDeploy = () => {
        if (!isSaveDeployEnabled || isDeploying) {
            return;
        }

        if (!selectedDeviceId) {
            setDeployError('Device id is missing. Please select a DCD and try again.');
            return;
        }

        const deployBridge = async () => {
            try {
                setDeployError('');
                setIsDeploying(true);
                setDeployMessage('Establishing connection, please wait.');

                const parsedPort = Number.parseInt(String(bridgeConfig?.port || ''), 10);
                const location = getDeployLocationFromBridgeConfig();
                const deployPayload = {
                    name: bridgeConfig?.name || '',
                    connection: {
                        ip: bridgeConfig?.ipAddress || '',
                        port: Number.isFinite(parsedPort) ? parsedPort : 0
                    },
                    location,
                    meta: {
                        processingMode: 'streaming',
                        protocol: sanitizeProtocolMeta(bridgeConfig?.protocol)
                    },
                    templateInfo: {
                        variables: [
                            {
                                label: 'location_path',
                                value: getLocationPathValue(location)
                            }
                        ],
                        rootUUID: '00000000-0000-0000-0000-000000000000',
                        isTemplated: false
                    }
                };

                const deployResponse = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        Accept: 'application/json'
                    },
                    credentials: 'include',
                    body: JSON.stringify({ payload: deployPayload })
                });

                const deployData = await deployResponse.json().catch(() => ({}));

                if (!deployResponse.ok) {
                    throw new Error(getErrorMessage(deployData, 'Failed to queue protocol converter deployment.'));
                }

                const deployActionId = extractActionId(deployData);

                if (!deployActionId) {
                    throw new Error('Protocol converter deploy request was queued but action id was not found.');
                }

                const deployResult = await pollProtocolConverterActionResult(deployActionId);
                const converterUuid = extractConverterUuid(deployResult);

                if (!converterUuid) {
                    throw new Error('Protocol converter was deployed but uuid was not found in action result.');
                }

                setDeployMessage('Adding configurations, please wait.');

                const editPayload = {
                    uuid: converterUuid,
                    name: bridgeConfig?.name || '',
                    connection: {
                        ip: bridgeConfig?.ipAddress || '',
                        port: Number.isFinite(parsedPort) ? parsedPort : 0
                    },
                    location,
                    readDFC: {
                        inputs: {
                            type: String(bridgeConfig?.readInputType || bridgeConfig?.protocol || 'modbus').toLowerCase(),
                            data: toMultilineString(bridgeConfig?.readInputYaml)
                        },
                        pipeline: {
                            threads: -1,
                            processors: {
                                '0': {
                                    type: String(bridgeConfig?.readProcessorType || 'tag_processor'),
                                    data: toMultilineString(bridgeConfig?.readProcessorYaml)
                                }
                            }
                        },
                        rawYAML: {
                            data: toMultilineString(bridgeConfig?.readRawYamlInject, 'buffer:\n  none: {}')
                        },
                        ignoreErrors: false
                    }
                };

                const editActionId = await queueProtocolConverterUpdate(converterUuid, editPayload);
                await pollProtocolConverterActionResult(editActionId);

                const query = new URLSearchParams();

                if (selectedDeviceId) {
                    query.set('deviceId', selectedDeviceId);
                }

                if (selectedDeviceName) {
                    query.set('deviceName', selectedDeviceName);
                }

                router.push(`/bridges/list${query.toString() ? `?${query.toString()}` : ''}`);
            } catch (error) {
                setDeployError(error.message || 'Failed to deploy bridge.');
            } finally {
                setIsDeploying(false);
                setDeployMessage('');
            }
        };

        deployBridge();
    };

    return (
        <div className="bridge-config-container">
            <div className="bridge-config-header">
                <div className="bridge-config-header-left">
                    <button className="bridge-config-back-btn" onClick={handleBack}>
                        <ArrowLeft size={22} />
                    </button>

                    <div>
                        <h1 className="bridge-config-title">Bridge Configuration</h1>
                        <p className="bridge-config-subtitle">
                            Connect data sources to the Unified Namespace
                        </p>
                    </div>
                </div>

                <button
                    className="bridge-config-save-btn"
                    onClick={handleSaveDeploy}
                    disabled={!isSaveDeployEnabled || isDeploying}
                >
                    <CloudUpload size={18} />
                    {isDeploying ? 'Deploying...' : 'Save & Deploy'}
                </button>
            </div>

            {deployError && (
                <div className="bridge-config-error-msg">
                    {deployError}
                </div>
            )}

            <div className="bridge-config-stepper">
                <div className="bridge-config-step-item completed">
                    <div className="bridge-config-step-circle">1</div>
                    <p>Choose Starting Point</p>
                </div>

                <div className="bridge-config-step-line full-active"></div>

                <div className="bridge-config-step-item active">
                    <div className="bridge-config-step-circle">2</div>
                    <p>Configure Bridge</p>
                </div>

                <div
                    className={`bridge-config-step-line ${
                        activeTab === 'connection'
                            ? 'progress-42'
                            : activeTab === 'readflow'
                            ? isStep3Ready
                                ? 'full-active'
                                : 'progress-90'
                            : ''
                    }`}
                ></div>

                <div className={`bridge-config-step-item ${isStep3Active ? 'active' : ''}`}>
                    <div className="bridge-config-step-circle">3</div>
                    <p>Review & Create</p>
                </div>
            </div>

            <div className="bridge-config-tabs">
                <button
                    className={`bridge-config-tab ${activeTab === 'general' ? 'active' : ''}`}
                    onClick={() => setActiveTab('general')}
                >
                    <FileText size={20} />
                    General
                </button>

                <button
                    className={`bridge-config-tab ${activeTab === 'connection' ? 'active' : ''}`}
                    onClick={() => setActiveTab('connection')}
                >
                    <Link size={20} />
                    Connection
                </button>

                <button
                    className={`bridge-config-tab ${activeTab === 'readflow' ? 'active' : ''}`}
                    onClick={() => setActiveTab('readflow')}
                >
                    <Network size={20} />
                    Read Flow
                </button>
            </div>

            <div className="bridge-config-body">
                {activeTab === 'general' && (
                    <General
                        bridgeConfig={bridgeConfig}
                        setBridgeConfig={setBridgeConfig}
                    />
                )}

                {activeTab === 'connection' && (
                    <Connection
                        bridgeConfig={bridgeConfig}
                        setBridgeConfig={setBridgeConfig}
                    />
                )}

                {activeTab === 'readflow' && (
                    <Readflow
                        bridgeConfig={bridgeConfig}
                        setBridgeConfig={setBridgeConfig}
                    />
                )}
            </div>

            {isDeploying && (
                <div className="bridge-config-queue-overlay">
                    <div className="bridge-config-queue-modal">
                        <div className="bridge-config-loader"></div>
                        <h3>{deployMessage}</h3>
                        <p>The deployment action has been queued and we are waiting for the device response.</p>
                    </div>
                </div>
            )}
        </div>
    );
};

export default BridgeConfiguration;