'use client';

import React, { useMemo, useState } from 'react';
import { ArrowLeft, ChevronRight, Filter, Search } from 'lucide-react';
import { useRouter } from 'next/navigation';
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
    const [searchQuery, setSearchQuery] = useState('');

    const filteredTemplates = useMemo(() => {
        const query = searchQuery.trim().toLowerCase();

        if (!query) {
            return templates;
        }

        return templates.filter((template) =>
            template.title.toLowerCase().includes(query) ||
            template.description.toLowerCase().includes(query) ||
            template.protocol.toLowerCase().includes(query)
        );
    }, [searchQuery]);

    const handleBack = () => {
        router.push('/dashboard/bridge/add');
    };

    const handleTemplateSelect = (template) => {
        router.push(`/dashboard/bridge/configure?protocol=${encodeURIComponent(template.protocol)}`);
    };

    const handleFilter = () => {
        // Filter logic will be added later
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

                <div className="template-step-line half-active"></div>

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

            <div className="template-divider"></div>

            <div className="template-controls-row">
                <div className="template-section-title">
                    <h2>Available Templates</h2>
                    <span>({templates.length} templates)</span>
                </div>

                <div className="template-actions">
                    <div className="template-search-wrapper">
                        <Search size={20} className="template-search-icon" />
                        <input
                            type="text"
                            placeholder="Search templates..."
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="template-search-input"
                        />
                    </div>

                    <button className="template-filter-btn" onClick={handleFilter}>
                        <Filter size={20} />
                        Filter
                    </button>
                </div>
            </div>

            <div className="template-list">
                {filteredTemplates.length > 0 ? (
                    filteredTemplates.map((template) => (
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
                    ))
                ) : (
                    <div className="template-empty-state">
                        No templates found.
                    </div>
                )}
            </div>
        </div>
    );
};

export default SelectTemplates;