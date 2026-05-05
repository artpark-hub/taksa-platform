'use client';

import React from 'react';
import { ArrowLeft, ChevronRight } from 'lucide-react';
import { useRouter, useSearchParams } from 'next/navigation';
import './SelectTemplates.css';

const templates = [
    {
        id: 'modbus-tcp',
        title: 'Modbus TCP',
        description: 'Read data from Modbus TCP devices, PLCs, drives, and industrial automation equipment.',
        flowType: 'Read Flow',
        protocol: 'Modbus'
    },
    {
        id: 'opcua',
        title: 'OPCUA',
        description: 'Read data from OPCUA servers, machines, gateways, and industrial control systems.',
        flowType: 'Read Flow',
        protocol: 'OPCUA'
    }
];

const SelectTemplates = () => {
    const router = useRouter();
    const searchParams = useSearchParams();
    const deviceId = searchParams?.get('deviceId') || '';
    const deviceName = searchParams?.get('deviceName') || '';

    const getDeviceQueryString = () => {
        const query = new URLSearchParams();

        if (deviceId) {
            query.set('deviceId', deviceId);
        }

        if (deviceName) {
            query.set('deviceName', deviceName);
        }

        return query.toString();
    };

    const handleBack = () => {
        const queryString = getDeviceQueryString();
        router.push(`/dashboard/bridges/add${queryString ? `?${queryString}` : ''}`);
    };

    const handleTemplateSelect = (template) => {
        const query = new URLSearchParams();
        query.set('protocol', template.protocol);

        if (deviceId) {
            query.set('deviceId', deviceId);
        }

        if (deviceName) {
            query.set('deviceName', deviceName);
        }

        router.push(`/dashboard/bridges/configure?${query.toString()}`);
    };

    return (
        <div className="select-template-container">
            <div className="select-template-header">
                <button className="select-template-back-btn" onClick={handleBack}>
                    <ArrowLeft size={22} />
                </button>

                <div>
                    <h1 className="select-template-title">Select Templates</h1>
                    <p className="select-template-subtitle">
                        Choose a template to start your bridge configuration
                    </p>
                </div>
            </div>

            <div className="template-stepper">
                <div className="template-step-item active">
                    <div className="template-step-circle">1</div>
                    <p>Choose Starting Point</p>
                </div>

                <div className="template-step-line progress-68"></div>

                <div className="template-step-item">
                    <div className="template-step-circle">2</div>
                    <p>Configure Bridge</p>
                </div>

                <div className="template-step-line"></div>

                <div className="template-step-item">
                    <div className="template-step-circle">3</div>
                    <p>Review & Create</p>
                </div>
            </div>

            <div className="template-controls-row">
                <div className="template-section-title">
                    <h2>Available Templates</h2>
                    <span>({templates.length} templates)</span>
                </div>
            </div>

            <div className="template-list">
                {templates.map((template) => (
                    <button
                        key={template.id}
                        className="template-card"
                        onClick={() => handleTemplateSelect(template)}
                    >
                        <div className="template-card-content">
                            <h3>{template.title}</h3>
                            <p>{template.description}</p>

                            <div className="template-tags">
                                <span>{template.flowType}</span>
                                <span>{template.protocol}</span>
                            </div>
                        </div>

                        <ChevronRight size={30} className="template-chevron" />
                    </button>
                ))}
            </div>
        </div>
    );
};

export default SelectTemplates;