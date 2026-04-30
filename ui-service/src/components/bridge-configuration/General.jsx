'use client';

import React from 'react';
import { ChevronDown, Plus } from 'lucide-react';
import './General.css';

const General = ({ bridgeConfig, setBridgeConfig }) => {
    const handleInputChange = (event) => {
        const { name, value } = event.target;

        setBridgeConfig((prev) => ({
            ...prev,
            [name]: value
        }));
    };

    const handleAddLevel = () => {
        // Add level logic will be added later
    };

    return (
        <div className="bridge-general-card">
            <div className="bridge-general-card-header">
                <h2>General Information</h2>
                <p>Name and organize this bridge in your hierarchy.</p>
            </div>

            <div className="bridge-general-form">
                <div className="bridge-general-form-row">
                    <label>
                        Name
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        name="name"
                        value={bridgeConfig.name}
                        onChange={handleInputChange}
                    />
                </div>

                <div className="bridge-general-form-row">
                    <label>
                        Instance
                        <span>*</span>
                    </label>

                    <div className="bridge-general-select-wrapper">
                        <select
                            name="instance"
                            value={bridgeConfig.instance}
                            onChange={handleInputChange}
                        >
                            <option value="artparktest">artparktest</option>
                        </select>

                        <ChevronDown size={18} className="bridge-general-select-icon" />
                    </div>
                </div>

                <div className="bridge-general-form-row">
                    <label>
                        Level 0
                        <span>*</span>
                    </label>

                    <input
                        type="text"
                        name="level0"
                        value={bridgeConfig.level0}
                        onChange={handleInputChange}
                    />
                </div>

                <button className="bridge-general-add-level-btn" onClick={handleAddLevel}>
                    <Plus size={20} />
                    Add Level 1
                </button>
            </div>
        </div>
    );
};

export default General;