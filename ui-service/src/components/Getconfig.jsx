'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { AlertTriangle, ArrowLeft } from 'lucide-react';
import './Getconfig.css';

const Getconfig = () => {
    const router = useRouter();
    const searchParams = useSearchParams();
    const editorRef = useRef(null);

    const deviceId = searchParams?.get('deviceId') || '';
    const deviceName = searchParams?.get('deviceName') || 'Unknown device';

    const [configValue, setConfigValue] = useState('');
    const [initialConfigValue, setInitialConfigValue] = useState('');
    const [configLastModifiedTime, setConfigLastModifiedTime] = useState('');
    const [isLoading, setIsLoading] = useState(true);
    const [isSaving, setIsSaving] = useState(false);
    const [isEditing, setIsEditing] = useState(false);
    const [isQueued, setIsQueued] = useState(false);
    const [queueMessage, setQueueMessage] = useState('');
    const [errorMessage, setErrorMessage] = useState('');
    const [successMessage, setSuccessMessage] = useState('');

    const hasChanges = useMemo(() => configValue !== initialConfigValue, [configValue, initialConfigValue]);

    const getErrorMessage = (data, fallback) => {
        return (
            data?.error?.message ||
            data?.message ||
            data?.details ||
            data?.error ||
            fallback
        );
    };

    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

    const pollConfigActionResult = async (actionId, cancelledRef) => {
        for (let attempt = 0; attempt < 60; attempt += 1) {
            if (cancelledRef?.current) {
                return null;
            }

            const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/config/${encodeURIComponent(actionId)}/result`, {
                method: 'GET',
                headers: {
                    Accept: 'application/json'
                },
                credentials: 'include'
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to get config action result.'));
            }

            const status = String(data?.status || '').toUpperCase();

            if (status === 'COMPLETED') {
                return data;
            }

            if (status === 'FAILED') {
                throw new Error(getErrorMessage(data, 'Config action failed.'));
            }

            await wait(3000);
        }

        throw new Error('Config request timed out. Please try again.');
    };

    const queueGetConfig = async (cancelledRef) => {
        const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/config`, {
            method: 'GET',
            headers: {
                Accept: 'application/json'
            },
            credentials: 'include'
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to queue config request.'));
        }

        if (!data?.actionId) {
            throw new Error('Config request was queued but action id was not found.');
        }

        const result = await pollConfigActionResult(data.actionId, cancelledRef);
        if (!result || cancelledRef.current) {
            return;
        }

        const fetchedConfig = result?.getConfigResponse?.content;
        const lastModifiedTime = result?.getConfigResponse?.lastModifiedTime;

        if (typeof fetchedConfig !== 'string') {
            throw new Error('Config action completed but config content was not found.');
        }

        if (!lastModifiedTime) {
            throw new Error('Config action completed but last modified time was not found.');
        }

        setConfigValue(fetchedConfig);
        setInitialConfigValue(fetchedConfig);
        setConfigLastModifiedTime(lastModifiedTime);
    };

    useEffect(() => {
        const cancelledRef = { current: false };

        const loadConfig = async () => {
            if (!deviceId) {
                if (!cancelledRef.current) {
                    setConfigValue('');
                    setInitialConfigValue('');
                    setConfigLastModifiedTime('');
                    setIsLoading(false);
                    setErrorMessage('Device id is missing.');
                }
                return;
            }

            try {
                setIsLoading(true);
                setIsQueued(true);
                setQueueMessage('Fetching Config file from device, please wait.');
                setErrorMessage('');
                setSuccessMessage('');

                await queueGetConfig(cancelledRef);
            } catch (error) {
                if (!cancelledRef.current) {
                    setConfigValue('');
                    setInitialConfigValue('');
                    setConfigLastModifiedTime('');
                    setErrorMessage(error.message || 'Failed to load config file.');
                }
            } finally {
                if (!cancelledRef.current) {
                    setIsQueued(false);
                    setQueueMessage('');
                    setIsLoading(false);
                }
            }
        };

        loadConfig();

        return () => {
            cancelledRef.current = true;
        };
    }, [deviceId]);

    const handleSave = async () => {
        if (!hasChanges || isSaving || isLoading || !deviceId) {
            return;
        }

        if (!configLastModifiedTime) {
            setErrorMessage('Last modified time is missing. Please refresh the config before saving.');
            return;
        }

        const cancelledRef = { current: false };

        try {
            setIsSaving(true);
            setIsQueued(true);
            setQueueMessage('Setting Config file, kindly wait.');
            setErrorMessage('');
            setSuccessMessage('');

            const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/config`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'application/json'
                },
                credentials: 'include',
                body: JSON.stringify({
                    payload: {
                        content: configValue,
                        lastModifiedTime: configLastModifiedTime
                    }
                })
            });

            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to queue config save request.'));
            }

            if (!data?.actionId) {
                throw new Error('Config save request was queued but action id was not found.');
            }

            const result = await pollConfigActionResult(data.actionId, cancelledRef);
            if (!result || cancelledRef.current) {
                return;
            }

            setQueueMessage('Fetching Config file from device, please wait.');
            await queueGetConfig(cancelledRef);

            setSuccessMessage('Config saved successfully.');
        } catch (error) {
            setErrorMessage(error.message || 'Failed to save config file.');
        } finally {
            setIsQueued(false);
            setQueueMessage('');
            setIsSaving(false);
        }
    };

    const handleKeyDown = (event) => {
        if (event.key !== 'Tab') {
            return;
        }

        event.preventDefault();

        const textarea = event.target;
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const indentation = '  ';

        const updatedValue = `${configValue.slice(0, start)}${indentation}${configValue.slice(end)}`;
        setConfigValue(updatedValue);

        requestAnimationFrame(() => {
            textarea.selectionStart = start + indentation.length;
            textarea.selectionEnd = start + indentation.length;
        });
    };

    return (
        <div className="getconfig-page">
            <div className="getconfig-header-row">
                <button type="button" className="getconfig-back-btn" onClick={() => router.back()} aria-label="Go back">
                    <ArrowLeft size={18} />
                </button>

                <div className="getconfig-title-wrap">
                    <h1>Config File</h1>
                    <p>View and edit the configuration file for your UMH Core instance.</p>
                </div>

                <button
                    type="button"
                    className="getconfig-save-btn"
                    onClick={handleSave}
                    disabled={!hasChanges || isSaving || isLoading || !deviceId}
                >
                    {isSaving ? 'Saving...' : 'Save'}
                </button>
            </div>

            {errorMessage && <div className="getconfig-error-msg">{errorMessage}</div>}
            {successMessage && <div className="getconfig-success-msg">{successMessage}</div>}

            <div className="getconfig-warning-box" role="alert">
                <AlertTriangle size={18} />
                <div>
                    <strong>Warning</strong>
                    <p>
                        You are editing the config.yaml file directly for <span>{deviceName}</span>. Use this feature with caution. A wrong config may break the system and it may only be recoverable via direct SSH access.
                    </p>
                </div>
            </div>

            <div className="getconfig-editor-shell" onClick={() => {
                setIsEditing(true);
                editorRef.current?.focus();
            }}>
                {!isEditing && !isLoading && <span className="getconfig-edit-label">Click to edit</span>}
                <textarea
                    ref={editorRef}
                    className="getconfig-editor"
                    value={isLoading ? 'Loading config...' : configValue}
                    onChange={(e) => setConfigValue(e.target.value)}
                    onFocus={() => setIsEditing(true)}
                    onBlur={() => setIsEditing(false)}
                    onKeyDown={handleKeyDown}
                    spellCheck={false}
                    disabled={isLoading}
                />
            </div>

            {isQueued && (
                <div className="getconfig-queue-overlay">
                    <div className="getconfig-queue-modal">
                        <div className="getconfig-loader"></div>
                        <h3>{queueMessage}</h3>
                        <p>The config action has been queued and we are waiting for the device response.</p>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Getconfig;