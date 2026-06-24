'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ArrowLeft, CloudUpload, FileText, Link, Network } from 'lucide-react';
import { useRouter, useSearchParams } from 'next/navigation';
import EditGeneral from './Edit-general';
import EditConnection from './Edit-connection';
import EditReadflow from './Edit-readflow';
import './EditBridge.css';
import { buildOpcUaFacadeRequest, isOpcUaProtocol, opcUaFacadeResultToBridgeConfig } from '../../lib/opcuaFacade';

const EditBridge = () => {
    const router = useRouter();
    const searchParams = useSearchParams();

    const bridgeId = searchParams.get('bridgeId') || '';
    const selectedDeviceId = searchParams.get('deviceId') || '';
    const selectedDeviceName = searchParams.get('deviceName') || '';
    const bridgeType = searchParams.get('bridgeType') || '';

    const [activeTab, setActiveTab] = useState('general');
    const [visitedTabs, setVisitedTabs] = useState({ general: true });
    const [isDeploying, setIsDeploying] = useState(false);
    const [deployMessage, setDeployMessage] = useState('');
    const [deployError, setDeployError] = useState('');
    const [deployTimer, setDeployTimer] = useState(0);
    const [isLoadingConfig, setIsLoadingConfig] = useState(true);
    const [configLoadTimer, setConfigLoadTimer] = useState(0);
    const deployCountdown = Math.max(60 - deployTimer, 0);
    const configLoadCountdown = Math.max(45 - configLoadTimer, 0);

    const [bridgeConfig, setBridgeConfig] = useState({
        name: '',
        instance: selectedDeviceName,
        level0: '',
        levels: [],
        protocol: 'Modbus',
        dataType: 'Time Series',
        ipAddress: '',
        port: '',
        readInputType: 'modbus',
        readInputYaml: '',
        readProcessorType: 'tag_processor',
        readProcessorYaml: '',
        readRawYamlInject: 'buffer:\n  none: {}',
        metaProtocol: 'modbus_tcp',
        templateVariables: [],
        readTemplateVariables: []
    });
    const initialBridgeConfigRef = useRef(null);
    const loadedProtocolConverterRef = useRef(null);

    const normalizeBridgeConfig = (config) => {
        const levels = Array.isArray(config?.levels)
            ? config.levels
                  .map((level) => ({
                      index: Number(level?.index ?? 0),
                      value: String(level?.value ?? '').trim()
                  }))
                  .sort((a, b) => a.index - b.index)
            : [];

        const templateVariables = Array.isArray(config?.templateVariables)
            ? config.templateVariables.map((item) => ({
                  label: String(item?.label ?? '').trim(),
                  value: String(item?.value ?? '').trim()
              }))
            : [];
        const readTemplateVariables = Array.isArray(config?.readTemplateVariables)
            ? config.readTemplateVariables.map((item) => ({
                  label: String(item?.label ?? '').trim(),
                  value: String(item?.value ?? '').trim()
              }))
            : [];

        return {
            name: String(config?.name ?? '').trim(),
            level0: String(config?.level0 ?? '').trim(),
            levels,
            ipAddress: String(config?.ipAddress ?? '').trim(),
            port: String(config?.port ?? '').trim(),
            readInputType: String(config?.readInputType ?? '').trim().toLowerCase(),
            readInputYaml: String(config?.readInputYaml ?? ''),
            readProcessorType: String(config?.readProcessorType ?? '').trim().toLowerCase(),
            readProcessorYaml: String(config?.readProcessorYaml ?? ''),
            readRawYamlInject: String(config?.readRawYamlInject ?? ''),
            metaProtocol: String(config?.metaProtocol ?? '').trim().toLowerCase(),
            templateVariables,
            readTemplateVariables
        };
    };

    const getErrorMessage = (data, fallback) => {
        return (
            data?.error?.message ||
            data?.message ||
            data?.details ||
            fallback
        );
    };

    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

    const sanitizeProtocolMeta = (protocol) => {
        const normalized = String(protocol || '')
            .trim()
            .toLowerCase()
            .replace(/[\s_-]/g, '');

        if (normalized === 'modbus' || normalized === 'modbustcp') {
            return 'modbus_tcp';
        }

        if (normalized === 'opcua' || normalized === 'benthosopcua') {
            return 'opcua';
        }

        return normalized || 'modbus_tcp';
    };

    const toMultilineString = (value, fallback = '') => {
        const normalized = String(value ?? fallback);
        return normalized.endsWith('\n') ? normalized : `${normalized}\n`;
    };

    const getLocationPathValue = (location) => {
        if (!location || typeof location !== 'object') {
            return 'default-location';
        }

        const locationPath = Object.entries(location)
            .filter(([key]) => /^\d+$/.test(String(key)))
            .sort((a, b) => Number(a[0]) - Number(b[0]))
            .map(([, value]) => String(value ?? '').trim())
            .filter(Boolean)
            .join('/');

        return locationPath || 'default-location';
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

        return normalizedLevels.reduce(
            (acc, level) => {
                if (Number(level.index) > 0) {
                    acc[String(level.index)] = String(level?.value || '');
                }
                return acc;
            },
            {
                '0': String(bridgeConfig?.level0 || '')
            }
        );
    };

    useEffect(() => {
        if (!isDeploying) {
            setDeployTimer(0);
            return;
        }

        setDeployTimer(0);
        const interval = setInterval(() => {
            setDeployTimer((prev) => prev + 1);
        }, 1000);

        return () => clearInterval(interval);
    }, [isDeploying]);

    useEffect(() => {
        if (!isLoadingConfig) {
            setConfigLoadTimer(0);
            return;
        }

        setConfigLoadTimer(0);
        const interval = setInterval(() => {
            setConfigLoadTimer((prev) => prev + 1);
        }, 1000);

        return () => clearInterval(interval);
    }, [isLoadingConfig]);

    const pollProtocolConverterActionResult = async (
        actionId,
        maxWaitSeconds = 45,
        resultType = 'generic',
        { requireResult = true } = {}
    ) => {
        const maxAttempts = Math.ceil((maxWaitSeconds * 1000) / 2000);
        const isOpcUaResult = resultType === 'opcua';
        
        for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
            const resultPath = isOpcUaResult
                ? `/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/opc-ua/actions/${encodeURIComponent(actionId)}/result`
                : `/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/${encodeURIComponent(actionId)}/result`;
            const response = await fetch(resultPath, {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to get bridge configuration.'));
            }

            const statusText = String(data?.status ?? '').toUpperCase();
            const hasCompletedAt = Boolean(data?.completedAt);
            const hasError = Boolean(data?.errorMessage);
            const hasResult = Boolean(data?.result);
            const isFailed = hasError || statusText.includes('FAILED') || ['5', '6', '7', '8'].includes(statusText);
            const isCompleted = statusText.includes('COMPLETED') || statusText === '4' || hasCompletedAt;

            if (isFailed) {
                const errMsg = data?.errorMessage || data?.error?.message || data?.message || 'Bridge configuration retrieval failed.';
                throw new Error(errMsg);
            }

            if (hasResult || isCompleted) {
                if (requireResult && !hasResult) {
                    throw new Error('Bridge configuration retrieval completed without result data.');
                }
                return data;
            }

            await wait(2000);
        }

        throw new Error(`Getting bridge configuration timed out after ${maxWaitSeconds} seconds. Please try again.`);
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

    const queueOpcUaProtocolConverterUpdate = async (converterUuid, editPayload) => {
        const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/opc-ua/${encodeURIComponent(converterUuid)}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify(editPayload)
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to queue OPC-UA bridge edit request.'));
        }

        const actionId = extractActionId(data);

        if (!actionId) {
            throw new Error('OPC-UA bridge edit was queued but action id was not found.');
        }

        return actionId;
    };

    const queueOpcUaProtocolConverterGet = async (converterUuid) => {
        const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/opc-ua/${encodeURIComponent(converterUuid)}`, {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to queue OPC-UA bridge configuration retrieval.'));
        }

        const actionId = extractActionId(data);

        if (!actionId) {
            throw new Error('OPC-UA bridge configuration retrieval was queued but action id was not found.');
        }

        return actionId;
    };

    useEffect(() => {
        let cancelled = false;

        const loadBridgeData = async () => {
            if (!bridgeId || !selectedDeviceId) {
                setIsLoadingConfig(false);
                return;
            }

            try {
                const useOpcUaFacade = isOpcUaProtocol(bridgeType);
                const controller = new AbortController();
                const timeoutId = setTimeout(() => controller.abort(), 10000);

                let initResponse;
                try {
                    const initPath = useOpcUaFacade
                        ? `/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/opc-ua/${encodeURIComponent(bridgeId)}`
                        : `/api/v1/devicemgmt/devices/${encodeURIComponent(selectedDeviceId)}/protocol-converters/${encodeURIComponent(bridgeId)}`;
                    initResponse = await fetch(initPath, {
                        method: 'GET',
                        headers: {
                            Accept: 'application/json'
                        },
                        credentials: 'include',
                        signal: controller.signal
                    });
                } finally {
                    clearTimeout(timeoutId);
                }

                const initData = await initResponse.json().catch(() => ({}));

                if (!initResponse.ok) {
                    throw new Error(getErrorMessage(initData, 'Failed to initiate bridge configuration retrieval.'));
                }

                const actionId = extractActionId(initData);

                if (!actionId) {
                    throw new Error('Configuration retrieval was initiated but action id was not found.');
                }

                if (cancelled) return;

                const resultData = await pollProtocolConverterActionResult(
                    actionId,
                    45,
                    useOpcUaFacade ? 'opcua' : 'generic',
                    { requireResult: true }
                );

                if (cancelled) return;

                if (useOpcUaFacade) {
                    const nextConfig = opcUaFacadeResultToBridgeConfig({
                        result: resultData?.result || resultData,
                        selectedDeviceName,
                        fallbackBridgeId: bridgeId
                    });
                    loadedProtocolConverterRef.current = resultData?.result || resultData;
                    setBridgeConfig(nextConfig);
                    initialBridgeConfigRef.current = normalizeBridgeConfig(nextConfig);
                    return;
                }

                const result = resultData?.result || resultData;
                loadedProtocolConverterRef.current = result && typeof result === 'object' ? result : null;
                const levels = result?.location || {};

                const normalizedLevels = Object.entries(levels)
                    .filter(([key]) => /^\d+$/.test(String(key)) && String(key) !== '0')
                    .sort((a, b) => Number(a[0]) - Number(b[0]))
                    .map(([key, value], idx) => ({
                        key: `level${key}`,
                        index: Number(key),
                        label: `Level ${key}`,
                        value: String(value ?? ''),
                        isUserAdded: idx > 0
                    }));

                const processorData = result?.readDFC?.pipeline?.processors?.['0']?.data || '';
                const inputData = result?.readDFC?.inputs?.data || '';
                const rawYamlData = result?.readDFC?.rawYAML?.data || 'buffer:\n  none: {}';

                const rawMetaProtocol = sanitizeProtocolMeta(result?.meta?.protocol || 'modbus_tcp');
                const loadedInputType = String(result?.readDFC?.inputs?.type || '').trim().toLowerCase();
                const resolvedInputType = loadedInputType || (rawMetaProtocol === 'opcua' ? 'opcua' : 'modbus');
                const resolvedProtocolLabel = rawMetaProtocol === 'opcua' ? 'OPCUA' : 'Modbus';
                const templateVariables = Array.isArray(result?.templateInfo?.variables)
                    ? result.templateInfo.variables
                    : [];

                // Sync templateInfo variables with current ip/port from connection
                const ip = result?.connection?.ip || '';
                const port = String(result?.connection?.port || '');
                const updatedVars = templateVariables.map((v) => {
                    if (String(v.label).toUpperCase() === 'IP') return { ...v, value: ip };
                    if (String(v.label).toUpperCase() === 'PORT') return { ...v, value: port };
                    return v;
                });

                const nextConfig = {
                    name: result?.name || '',
                    instance: selectedDeviceName,
                    level0: String(levels?.['0'] || ''),
                    levels: normalizedLevels,
                    protocol: resolvedProtocolLabel,
                    dataType: 'Time Series',
                    ipAddress: ip,
                    port: port,
                    readInputType: resolvedInputType,
                    readInputYaml: inputData,
                    readProcessorType: 'tag_processor',
                    readProcessorYaml: processorData,
                    readRawYamlInject: rawYamlData,
                    metaProtocol: rawMetaProtocol,
                    templateVariables: updatedVars,
                    readTemplateVariables: []
                };

                setBridgeConfig(nextConfig);
                initialBridgeConfigRef.current = normalizeBridgeConfig(nextConfig);
            } catch (error) {
                console.error('Error loading bridge data:', error);
                setDeployError(error.message || 'Failed to load bridge configuration.');
            } finally {
                if (!cancelled) {
                    setIsLoadingConfig(false);
                }
            }
        };

        loadBridgeData();

        return () => {
            cancelled = true;
        };
    }, [bridgeId, selectedDeviceId, selectedDeviceName, bridgeType]);

    const isStep2Ready = useMemo(() => {
        const hasName = String(bridgeConfig?.name || '').trim().length > 0;
        const hasLocation = String(bridgeConfig?.level0 || '').trim().length > 0;

        return hasName && hasLocation;
    }, [bridgeConfig?.name, bridgeConfig?.level0]);

    const isStep3Ready = useMemo(() => {
        const hasInput = String(bridgeConfig?.readInputYaml || '').trim().length > 0;
        const hasProcessor = String(bridgeConfig?.readProcessorYaml || '').trim().length > 0;

        return hasInput && hasProcessor;
    }, [bridgeConfig?.readInputYaml, bridgeConfig?.readProcessorYaml]);

    const hasChanges = useMemo(() => {
        if (!initialBridgeConfigRef.current) {
            return false;
        }

        return JSON.stringify(normalizeBridgeConfig(bridgeConfig)) !== JSON.stringify(initialBridgeConfigRef.current);
    }, [bridgeConfig]);

    const isBridgeConfigValid = useMemo(() => {
        const hasName = String(bridgeConfig?.name || '').trim().length > 0;
        const hasLevel0 = String(bridgeConfig?.level0 || '').trim().length > 0;
        const hasIp = String(bridgeConfig?.ipAddress || '').trim().length > 0;
        const hasInputYaml = String(bridgeConfig?.readInputYaml || '').trim().length > 0;
        const hasProcessorYaml = String(bridgeConfig?.readProcessorYaml || '').trim().length > 0;
        const parsedPort = Number.parseInt(String(bridgeConfig?.port || ''), 10);
        const hasValidPort = Number.isInteger(parsedPort) && parsedPort > 0 && parsedPort <= 65535;

        return hasName && hasLevel0 && hasIp && hasValidPort && hasInputYaml && hasProcessorYaml;
    }, [
        bridgeConfig?.name,
        bridgeConfig?.level0,
        bridgeConfig?.ipAddress,
        bridgeConfig?.port,
        bridgeConfig?.readInputYaml,
        bridgeConfig?.readProcessorYaml
    ]);

    const isOnReadflow = activeTab === 'readflow';
    const isStep3Active = isOnReadflow && isStep3Ready;
    const isSaveDeployEnabled = useMemo(() => {
        return hasChanges && isBridgeConfigValid && !isLoadingConfig;
    }, [hasChanges, isBridgeConfigValid, isLoadingConfig]);

    const handleBack = () => {
        router.back();
    };

    const handleTabChange = (tab) => {
        setActiveTab(tab);
        setVisitedTabs((prev) => (prev[tab] ? prev : { ...prev, [tab]: true }));
    };

    const handleSaveDeploy = () => {
        if (!isSaveDeployEnabled || isDeploying) {
            return;
        }

        if (!selectedDeviceId || !bridgeId) {
            setDeployError('Device or bridge id is missing. Please try again.');
            return;
        }

        if (!isBridgeConfigValid) {
            setDeployError('Please fill all required fields with valid values before updating.');
            return;
        }

        const deployBridge = async () => {
            try {
                setDeployError('');
                setIsDeploying(true);
                setDeployMessage('Updating Configuration, kindly wait.');

                const parsedPort = Number.parseInt(String(bridgeConfig?.port || ''), 10);
                const portNum = Number.isFinite(parsedPort) ? parsedPort : 0;
                const location = getDeployLocationFromBridgeConfig();

                const currentIp = bridgeConfig?.ipAddress || '';
                const currentPort = String(portNum);
                const protocolMeta = sanitizeProtocolMeta(bridgeConfig?.metaProtocol || bridgeConfig?.protocol);
                const normalizedReadInputType = String(bridgeConfig?.readInputType || protocolMeta || 'modbus')
                    .trim()
                    .toLowerCase()
                    .replace(/[\s_-]/g, '');
                const isOpcuaPatch = protocolMeta === 'opcua' || normalizedReadInputType === 'opcua' || normalizedReadInputType === 'benthosopcua';
                const readInputType = isOpcuaPatch ? 'benthos_opcua' : normalizedReadInputType;
                const readTemplateVariables = Array.isArray(bridgeConfig?.readTemplateVariables)
                    ? bridgeConfig.readTemplateVariables
                    : [];

                if (isOpcuaPatch) {
                    const editPayload = buildOpcUaFacadeRequest({
                        bridgeConfig,
                        deviceId: selectedDeviceId,
                        uuid: bridgeId,
                        location,
                        port: portNum
                    });

                    const actionId = await queueOpcUaProtocolConverterUpdate(bridgeId, editPayload);
                    setDeployMessage('Applying OPC-UA changes, please wait.');
                    await pollProtocolConverterActionResult(actionId, 60, 'opcua', { requireResult: false });

                    setDeployMessage('Refreshing OPC-UA configuration, please wait.');
                    const getActionId = await queueOpcUaProtocolConverterGet(bridgeId);
                    const refreshed = await pollProtocolConverterActionResult(getActionId, 60, 'opcua', { requireResult: true });
                    const nextConfig = opcUaFacadeResultToBridgeConfig({
                        result: refreshed?.result || refreshed,
                        selectedDeviceName,
                        fallbackBridgeId: bridgeId
                    });
                    loadedProtocolConverterRef.current = refreshed?.result || refreshed;
                    setBridgeConfig(nextConfig);
                    initialBridgeConfigRef.current = normalizeBridgeConfig(nextConfig);

                    setDeployMessage('Update completed successfully!');
                    setTimeout(() => {
                        setIsDeploying(false);
                        setDeployMessage('');
                        router.push(`/dashboard/bridges/list?deviceId=${encodeURIComponent(selectedDeviceId)}&deviceName=${encodeURIComponent(selectedDeviceName)}`);
                    }, 2000);
                    return;
                }

                const loadedConverter = loadedProtocolConverterRef.current && typeof loadedProtocolConverterRef.current === 'object'
                    ? loadedProtocolConverterRef.current
                    : {};
                const loadedConnection = loadedConverter?.connection || {};
                const loadedReadDFC = loadedConverter?.readDFC || {};
                const loadedPipeline = loadedReadDFC?.pipeline || {};
                const loadedProcessors = loadedPipeline?.processors || {};
                const loadedProcessor0 = loadedProcessors?.['0'] || {};
                const loadedTemplateInfo = loadedConverter?.templateInfo || {};
                const loadedMeta = loadedConverter?.meta || {};

                const editPayload = {
                    ...loadedConverter,
                    uuid: loadedConverter?.uuid || bridgeId,
                    name: bridgeConfig?.name || '',
                    connection: {
                        ...loadedConnection,
                        ip: currentIp,
                        port: portNum
                    },
                    location,
                    readDFC: {
                        ...loadedReadDFC,
                        ignoreErrors: !isOpcuaPatch,
                        inputs: {
                            ...(loadedReadDFC?.inputs || {}),
                            type: readInputType,
                            data: toMultilineString(bridgeConfig?.readInputYaml)
                        },
                        pipeline: {
                            ...loadedPipeline,
                            threads: -1,
                            processors: {
                                ...loadedProcessors,
                                '0': {
                                    ...loadedProcessor0,
                                    type: String(bridgeConfig?.readProcessorType || 'tag_processor').toLowerCase(),
                                    data: toMultilineString(bridgeConfig?.readProcessorYaml)
                                }
                            }
                        },
                        rawYAML: {
                            ...(loadedReadDFC?.rawYAML || {}),
                            data: toMultilineString(bridgeConfig?.readRawYamlInject, 'buffer:\n  none: {}')
                        }
                    },
                    meta: {
                        ...loadedMeta,
                        protocol: protocolMeta,
                        ...(isOpcuaPatch ? { processingMode: 'streaming' } : {})
                    }
                };

                if (isOpcuaPatch) {
                    if (readTemplateVariables.length > 0) {
                        editPayload.templateInfo = {
                            ...loadedTemplateInfo,
                            variables: readTemplateVariables
                        };
                    } else {
                        delete editPayload.templateInfo;
                    }
                } else {
                    const syncedVars = (bridgeConfig?.templateVariables || []).map((v) => {
                        if (String(v.label).toUpperCase() === 'IP') return { ...v, value: currentIp };
                        if (String(v.label).toUpperCase() === 'PORT') return { ...v, value: currentPort };
                        return v;
                    });

                    editPayload.templateInfo = {
                        ...loadedTemplateInfo,
                        variables: syncedVars
                    };
                }

                const actionId = await queueProtocolConverterUpdate(bridgeId, editPayload);
                setDeployMessage('Applying changes, please wait.');
                await pollProtocolConverterActionResult(actionId, 60, 'generic', { requireResult: false });

                setDeployMessage('Update completed successfully!');
                setTimeout(() => {
                    setIsDeploying(false);
                    setDeployMessage('');
                    router.push(`/dashboard/bridges/list?deviceId=${encodeURIComponent(selectedDeviceId)}&deviceName=${encodeURIComponent(selectedDeviceName)}`);
                }, 2000);
            } catch (error) {
                console.error('Failed to update bridge:', error);
                setDeployError(error.message || 'Failed to update bridge. Please try again.');
                setIsDeploying(false);
                setDeployMessage('');
            }
        };

        deployBridge();
    };

    return (
        <div className="bridge-config-container">
            {isLoadingConfig && (
                <div className="bridge-config-queue-overlay">
                    <div className="bridge-config-queue-modal">
                        <div className="bridge-config-loader"></div>
                        <h3>Getting config codes, kindly wait.</h3>
                        <p>Retrieving bridge configuration from the device.</p>
                        <p className="bridge-config-timer">Time left: {configLoadCountdown}s</p>
                    </div>
                </div>
            )}
            <div className="bridge-config-header">
                <div className="bridge-config-header-left">
                    <button className="bridge-config-back-btn" onClick={handleBack} aria-label="Back">
                        <ArrowLeft size={22} />
                    </button>

                    <div>
                        <h1 className="bridge-config-title">Edit Bridge Configuration</h1>
                        <p className="bridge-config-subtitle">
                            Modify your data source configuration
                        </p>
                    </div>
                </div>

                <button
                    className="bridge-config-save-btn"
                    onClick={handleSaveDeploy}
                    disabled={!isSaveDeployEnabled || isDeploying}
                >
                    <CloudUpload size={18} />
                    {isDeploying ? 'Updating...' : 'Update & Deploy'}
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
                    <p>Edit Configuration</p>
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
                    <p>Review & Update</p>
                </div>
            </div>

            <div className="bridge-config-tabs">
                <button
                    className={`bridge-config-tab ${activeTab === 'general' ? 'active' : ''}`}
                    onClick={() => handleTabChange('general')}
                >
                    <FileText size={20} />
                    General
                </button>

                <button
                    className={`bridge-config-tab ${activeTab === 'connection' ? 'active' : ''}`}
                    onClick={() => handleTabChange('connection')}
                >
                    <Link size={20} />
                    Connection
                </button>

                <button
                    className={`bridge-config-tab ${activeTab === 'readflow' ? 'active' : ''}`}
                    onClick={() => handleTabChange('readflow')}
                >
                    <Network size={20} />
                    Read Flow
                </button>
            </div>

            <div className="bridge-config-body">
                {!isLoadingConfig && visitedTabs.general && (
                    <div style={{ display: activeTab === 'general' ? 'block' : 'none' }}>
                        <EditGeneral
                            bridgeConfig={bridgeConfig}
                            setBridgeConfig={setBridgeConfig}
                        />
                    </div>
                )}

                {!isLoadingConfig && visitedTabs.connection && (
                    <div style={{ display: activeTab === 'connection' ? 'block' : 'none' }}>
                        <EditConnection
                            bridgeConfig={bridgeConfig}
                            setBridgeConfig={setBridgeConfig}
                        />
                    </div>
                )}

                {!isLoadingConfig && visitedTabs.readflow && (
                    <div style={{ display: activeTab === 'readflow' ? 'block' : 'none' }}>
                        <EditReadflow
                            bridgeConfig={bridgeConfig}
                            setBridgeConfig={setBridgeConfig}
                        />
                    </div>
                )}
            </div>

            {isDeploying && (
                <div className="bridge-config-queue-overlay">
                    <div className="bridge-config-queue-modal">
                        <div className="bridge-config-loader"></div>
                        <h3>{deployMessage}</h3>
                        <p>The update action has been queued and we are waiting for the device response.</p>
                        <p className="bridge-config-timer">Time left: {deployCountdown}s</p>
                    </div>
                </div>
            )}
        </div>
    );
};

export default EditBridge;
