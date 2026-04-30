'use client';

import React, { useMemo, useState } from 'react';
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

    const [activeTab, setActiveTab] = useState('general');

    const [bridgeConfig, setBridgeConfig] = useState({
        name: 'Generic-modbus-bridge-1',
        instance: 'artparktest',
        level0: 'artpark',
        protocol: initialProtocol,
        dataType: 'Time Series',
        ipAddress: '',
        port: '502'
    });

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
        router.push('/bridges/add');
    };

    const handleSaveDeploy = () => {
        if (!isSaveDeployEnabled) {
            return;
        }

        // Save and deploy logic will be added later
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
                    disabled={!isSaveDeployEnabled}
                >
                    <CloudUpload size={18} />
                    Save & Deploy
                </button>
            </div>

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
        </div>
    );
};

export default BridgeConfiguration;